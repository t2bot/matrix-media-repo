package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/howeyc/gopass"
	"github.com/jeffail/tunny"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/logging"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/common/runtime"
	"github.com/turt2live/matrix-media-repo/controllers/upload_controller"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/synapse"
)

type fetchRequest struct {
	media      *synapse.LocalMedia
	csApiUrl   string
	serverName string
}

func main() {
	postgresHost := flag.String("dbHost", "localhost", "The PostgresSQL hostname for your Synapse database")
	postgresPort := flag.Int("dbPort", 5432, "The port for your Synapse's PostgreSQL database")
	postgresUsername := flag.String("dbUsername", "synapse", "The username for your Synapse's PostgreSQL database")
	postgresPassword := flag.String("dbPassword", "", "The password for your Synapse's PostgreSQL database. Can be omitted to be prompted when run")
	postgresDatabase := flag.String("dbName", "synapse", "The name of your Synapse database")
	baseUrl := flag.String("baseUrl", "http://localhost:8008", "The base URL to access your homeserver with")
	serverName := flag.String("serverName", "localhost", "The name of your homeserver (eg: matrix.org)")
	configPath := flag.String("config", "media-repo.yaml", "The path to the media repo configuration (with the database section completed)")
	migrationsPath := flag.String("migrations", "./migrations", "The absolute path the media repo's migrations folder")
	numWorkers := flag.Int("workers", 1, "The number of workers to use when downloading media. Using multiple workers risks deduplication not working as efficiently.")
	flag.Parse()

	config.Path = *configPath
	config.Runtime.MigrationsPath = *migrationsPath

	var realPsqlPassword string
	if *postgresPassword == "" {
		fmt.Printf("Postgres password: ")
		pass, err := gopass.GetPasswd()
		if err != nil {
			panic(err)
		}
		realPsqlPassword = string(pass[:])
	} else {
		realPsqlPassword = *postgresPassword
	}

	err := logging.Setup(config.Get().General.LogDirectory)
	if err != nil {
		panic(err)
	}

	logrus.Info("Starting up...")
	runtime.RunStartupSequence()

	logrus.Info("Setting up for importing...")

	connectionString := "postgres://" + *postgresUsername + ":" + realPsqlPassword + "@" + *postgresHost + ":" + strconv.Itoa(*postgresPort) + "/" + *postgresDatabase + "?sslmode=disable"
	csApiUrl := *baseUrl
	if csApiUrl[len(csApiUrl)-1:] == "/" {
		csApiUrl = csApiUrl[:len(csApiUrl)-1]
	}

	logrus.Info("Connecting to synapse database...")
	synDb, err := synapse.OpenDatabase(connectionString)
	if err != nil {
		panic(err)
	}

	logrus.Info("Fetching all local media records from synapse...")
	records, err := synDb.GetAllMedia()
	if err != nil {
		panic(err)
	}

	logrus.Info(fmt.Sprintf("Downloading %d media records", len(records)))

	pool := tunny.NewFunc(*numWorkers, fetchMedia)

	numCompleted := 0
	lock := &sync.RWMutex{}
	onComplete := func(interface{}, error) {
		lock.Lock()
		numCompleted++
		percent := int((float32(numCompleted) / float32(len(records))) * 100)
		logrus.Info(fmt.Sprintf("%d/%d downloaded (%d%%)", numCompleted, len(records), percent))
		lock.Unlock()
	}

	for i := 0; i < len(records); i++ {
		percent := int((float32(i+1) / float32(len(records))) * 100)
		record := records[i]

		logrus.Info(fmt.Sprintf("Queuing %s (%d/%d %d%%)", record.MediaId, i+1, len(records), percent))
		go func() {
			result := pool.Process(&fetchRequest{media: record, serverName: *serverName, csApiUrl: csApiUrl})
			onComplete(result, nil)
		}()
	}

	for numCompleted < len(records) {
		logrus.Info("Waiting for import to complete...")
		time.Sleep(1 * time.Second)
	}

	logrus.Info("Import completed")
}

func fetchMedia(req interface{}) interface{} {
	payload := req.(*fetchRequest)
	record := payload.media
	ctx := rcontext.Initial()

	db := storage.GetDatabase().GetMediaStore(ctx)

	_, err := db.Get(payload.serverName, record.MediaId)
	if err == nil {
		logrus.Info("Media already downloaded: " + payload.serverName + "/" + record.MediaId)
		return nil
	}

	body, err := downloadMedia(payload.csApiUrl, payload.serverName, record.MediaId)
	if err != nil {
		logrus.Error(err.Error())
		return nil
	}
	defer body.Close()

	_, err = upload_controller.StoreDirect(body, -1, record.ContentType, record.UploadName, record.UserId, payload.serverName, record.MediaId, common.KindLocalMedia, ctx)
	if err != nil {
		logrus.Error(err.Error())
		return nil
	}

	return nil
}

func downloadMedia(baseUrl string, serverName string, mediaId string) (io.ReadCloser, error) {
	downloadUrl := baseUrl + "/_matrix/media/r0/download/" + serverName + "/" + mediaId
	resp, err := http.Get(downloadUrl)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("received status code " + strconv.Itoa(resp.StatusCode))
	}

	return resp.Body, nil
}
