package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/assets"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/logging"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/common/runtime"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/datastores"
	"github.com/turt2live/matrix-media-repo/homeserver_interop/synapse"
	"github.com/turt2live/matrix-media-repo/pipelines/pipeline_upload"
	"github.com/turt2live/matrix-media-repo/util/ids"
	"golang.org/x/crypto/ssh/terminal"
)

func main() {
	postgresHost := flag.String("dbHost", "localhost", "The PostgresSQL hostname for your Synapse database")
	postgresPort := flag.Int("dbPort", 5432, "The port for your Synapse's PostgreSQL database")
	postgresUsername := flag.String("dbUsername", "synapse", "The username for your Synapse's PostgreSQL database")
	postgresPassword := flag.String("dbPassword", "", "The password for your Synapse's PostgreSQL database. Can be omitted to be prompted when run")
	postgresDatabase := flag.String("dbName", "synapse", "The name of your Synapse database")
	baseUrl := flag.String("baseUrl", "http://localhost:8008", "The base URL to access your homeserver with")
	serverName := flag.String("serverName", "localhost", "The name of your homeserver (eg: matrix.org)")
	configPath := flag.String("config", "media-repo.yaml", "The path to the media repo configuration (configured for the media repo's database)")
	migrationsPath := flag.String("migrations", "./migrations", "The absolute path the media repo's migrations folder")
	numWorkers := flag.Int("workers", 10, "The number of workers to use when downloading media. Using multiple workers is recommended.")
	flag.Parse()

	// Override config path with config for Docker users
	configEnv := os.Getenv("REPO_CONFIG")
	if configEnv != "" {
		configPath = &configEnv
	}

	config.Runtime.IsImportProcess = true // prevents us from creating media by accident
	config.Path = *configPath

	defer assets.Cleanup()
	assets.SetupMigrations(*migrationsPath)

	if ids.GetMachineId() == 0 {
		panic(errors.New("expected custom machine ID for import process (unsafe to import as Machine 0)"))
	}

	var realPsqlPassword string
	if *postgresPassword == "" {
		if !terminal.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Println("Sorry, your terminal does not support reading passwords. Please supply a -dbPassword or use a different terminal.")
			fmt.Println("If you're on Windows, try using a plain Command Prompt window instead of a bash-like terminal.")
			os.Exit(1)
			return // for good measure
		}
		fmt.Printf("Postgres password: ")
		pass, err := terminal.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			panic(err)
		}
		realPsqlPassword = string(pass[:])
	} else {
		realPsqlPassword = *postgresPassword
	}

	err := logging.Setup(
		config.Get().General.LogDirectory,
		config.Get().General.LogColors,
		config.Get().General.JsonLogs,
		config.Get().General.LogLevel,
	)
	if err != nil {
		panic(err)
	}

	logrus.Info("Starting up...")
	runtime.RunStartupSequence()

	connectionString := "postgres://" + *postgresUsername + ":" + realPsqlPassword + "@" + *postgresHost + ":" + strconv.Itoa(*postgresPort) + "/" + *postgresDatabase + "?sslmode=disable"
	csApiUrl := *baseUrl
	if csApiUrl[len(csApiUrl)-1:] == "/" {
		csApiUrl = csApiUrl[:len(csApiUrl)-1]
	}

	logrus.Debug("Connecting to synapse database...")
	synDb, err := synapse.OpenDatabase(connectionString)
	if err != nil {
		panic(err)
	}

	logrus.Debug("Fetching all local media records from synapse...")
	records, err := synDb.GetAllMedia()
	if err != nil {
		panic(err)
	}

	logrus.Info(fmt.Sprintf("Downloading %d media records", len(records)))

	pool, err := ants.NewPool(*numWorkers, ants.WithOptions(ants.Options{
		ExpiryDuration:   1 * time.Hour,
		PreAlloc:         false,
		MaxBlockingTasks: 0, // no limit
		Nonblocking:      false,
		Logger:           &logging.SendToDebugLogger{},
		DisablePurge:     false,
		PanicHandler: func(err interface{}) {
			panic(err)
		},
	}))
	if err != nil {
		panic(err)
	}

	numCompleted := 0
	mu := &sync.RWMutex{}
	onComplete := func() {
		mu.Lock()
		numCompleted++
		percent := int((float32(numCompleted) / float32(len(records))) * 100)
		logrus.Info(fmt.Sprintf("%d/%d downloaded (%d%%)", numCompleted, len(records), percent))
		mu.Unlock()
	}

	for i := 0; i < len(records); i++ {
		percent := int((float32(i+1) / float32(len(records))) * 100)
		record := records[i]

		logrus.Debug(fmt.Sprintf("Queuing %s (%d/%d %d%%)", record.MediaId, i+1, len(records), percent))
		err = pool.Submit(doWork(record, *serverName, csApiUrl, onComplete))
		if err != nil {
			panic(err)
		}
	}

	for numCompleted < len(records) {
		logrus.Debug("Waiting for import to complete...")
		time.Sleep(1 * time.Second)
	}

	logrus.Info("Import completed")
}

func doWork(record *synapse.LocalMedia, serverName string, csApiUrl string, onComplete func()) func() {
	return func() {
		defer onComplete()

		ctx := rcontext.Initial().LogWithFields(logrus.Fields{"origin": serverName, "mediaId": record.MediaId})

		db := database.GetInstance().Media.Prepare(ctx)

		dbRecord, err := db.GetById(serverName, record.MediaId)
		if err != nil {
			panic(err)
		}
		if dbRecord != nil {
			ctx.Log.Debug("Already downloaded - skipping")
			return
		}

		body, err := downloadMedia(csApiUrl, serverName, record.MediaId)
		if err != nil {
			panic(err)
		}

		dbRecord, err = pipeline_upload.Execute(ctx, serverName, record.MediaId, body, record.ContentType, record.UploadName, record.UserId, datastores.LocalMediaKind)
		if err != nil {
			panic(err)
		}

		if dbRecord.SizeBytes != record.SizeBytes {
			ctx.Log.Warnf("Size mismatch! Expected %d bytes but got %d", record.SizeBytes, dbRecord.SizeBytes)
		}
	}
}

func downloadMedia(baseUrl string, serverName string, mediaId string) (io.ReadCloser, error) {
	downloadUrl := baseUrl + "/_matrix/media/v3/download/" + serverName + "/" + mediaId
	resp, err := http.Get(downloadUrl)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("received status code " + strconv.Itoa(resp.StatusCode))
	}

	return resp.Body, nil
}
