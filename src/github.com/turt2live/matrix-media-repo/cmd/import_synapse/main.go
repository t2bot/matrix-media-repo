package main

import (
	"context"
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
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/logging"
	"github.com/turt2live/matrix-media-repo/services/media_service"
	"github.com/turt2live/matrix-media-repo/synapse"
)

type fetchRequest struct {
	media      *synapse.LocalMedia
	csApiUrl   string
	serverName string
}

func main() {
	postgresHost := flag.String("dbHost", "localhost", "The IP or hostname of the postgresql server with the synapse database")
	postgresPort := flag.Int("dbPort", 5432, "The port to access postgres on")
	postgresUsername := flag.String("dbUsername", "synapse", "The username to access postgres with")
	postgresPassword := flag.String("dbPassword", "", "The password to authorize the postgres user. Can be omitted to be prompted when run")
	postgresDatabase := flag.String("dbName", "synapse", "The name of the synapse database")
	baseUrl := flag.String("baseUrl", "http://localhost:8008", "The base URL to access your homeserver with")
	serverName := flag.String("serverName", "localhost", "The name of your homeserver (eg: matrix.org)")
	configPath := flag.String("config", "media-repo.yaml", "The path to the configuration")
	migrationsPath := flag.String("migrations", "./migrations", "The absolute path the migrations folder")
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

	pool, err := tunny.CreatePool(*numWorkers, fetchMedia).Open()
	if err != nil {
		panic(err)
	}

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
		pool.SendWorkAsync(&fetchRequest{media: record, serverName: *serverName, csApiUrl: csApiUrl}, onComplete)
	}

	for numCompleted < len(records)-1 {
		logrus.Info("Waiting for import to complete...")
		time.Sleep(1 * time.Second)
	}

	logrus.Info("Import completed")
}

func fetchMedia(req interface{}) interface{} {
	payload := req.(*fetchRequest)
	record := payload.media
	ctx := context.TODO()

	svc := media_service.New(ctx, logrus.WithFields(logrus.Fields{}))
	_, err := svc.GetMediaDirect(payload.serverName, record.MediaId)
	if err == nil {
		logrus.Info("Media already downloaded: " + payload.serverName + "/" + record.MediaId)
		return nil
	}

	body, err := downloadMedia(payload.csApiUrl, payload.serverName, record.MediaId)
	if err != nil {
		logrus.Error(err.Error())
		return nil
	}

	_, err = svc.StoreMedia(body, record.ContentType, record.UploadName, record.UserId, payload.serverName, record.MediaId)
	if err != nil {
		logrus.Error(err.Error())
		return nil
	}

	body.Close()
	return nil
}

func downloadMedia(baseUrl string, serverName string, mediaId string) (io.ReadCloser, error) {
	downloadUrl := baseUrl + "/_matrix/media/v1/download/" + serverName + "/" + mediaId
	resp, err := http.Get(downloadUrl)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("received status code " + strconv.Itoa(resp.StatusCode))
	}

	return resp.Body, nil
}
