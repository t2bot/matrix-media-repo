package main

import (
	"flag"
	"io"
	"os"
	"path"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/assets"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/logging"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/common/runtime"
	"github.com/turt2live/matrix-media-repo/controllers/data_controller"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
)

func main() {
	configPath := flag.String("config", "media-repo.yaml", "The path to the configuration")
	migrationsPath := flag.String("migrations", config.DefaultMigrationsPath, "The absolute path for the migrations folder")
	templatesPath := flag.String("templates", config.DefaultTemplatesPath, "The absolute path for the templates folder")
	entity := flag.String("entity", "", "The user ID or server name to export")
	destination := flag.String("destination", "./gdpr-data", "The directory for where export files should be placed")
	flag.Parse()

	// Override config path with config for Docker users
	configEnv := os.Getenv("REPO_CONFIG")
	if configEnv != "" {
		configPath = &configEnv
	}

	if *entity == "" {
		flag.Usage()
		os.Exit(1)
		return
	}

	config.Path = *configPath
	assets.SetupMigrations(*migrationsPath)
	assets.SetupTemplates(*templatesPath)

	var err error
	err = logging.Setup(config.Get().General.LogDirectory)
	if err != nil {
		panic(err)
	}

	logrus.Info("Starting up...")
	runtime.RunStartupSequence()

	logrus.Info("Starting export...")
	ctx := rcontext.Initial().LogWithFields(logrus.Fields{"flagEntity": *entity})
	var task *types.BackgroundTask
	var exportId string
	if (*entity)[0] == '@' {
		task, exportId, err = data_controller.StartUserExport(*entity, true, true, ctx)
	} else {
		task, exportId, err = data_controller.StartServerExport(*entity, true, true, ctx)
	}

	if err != nil {
		panic(err)
	}

	logrus.Info("Waiting for export to complete")
	waitChan := make(chan bool)
	defer close(waitChan)
	go func() {
		// Initial sleep to let the caches fill
		time.Sleep(1 * time.Second)

		ctx := rcontext.Initial().LogWithFields(logrus.Fields{"flagEntity": *entity, "async": true})
		db := storage.GetDatabase().GetMetadataStore(ctx)
		for true {
			ctx.Log.Info("Checking if task is complete")

			task, err := db.GetBackgroundTask(task.ID)
			if err != nil {
				logrus.Error(err)
			} else if task.EndTs > 0 {
				waitChan<-true
				return
			}

			time.Sleep(1 * time.Second)
		}
	}()
	<-waitChan

	logrus.Info("Export finished, dumping files")
	exportDb := storage.GetDatabase().GetExportStore(ctx)
	parts, err := exportDb.GetExportParts(exportId)
	if err != nil {
		panic(err)
	}

	// Create directory if not exists
	_ = os.MkdirAll(*destination, os.ModePerm)

	for _, p := range parts {
		s, err := datastore.DownloadStream(ctx, p.DatastoreID, p.Location)
		if err != nil {
			panic(err)
		}

		fname := path.Join(*destination, p.FileName)
		logrus.Info("Writing ", fname)
		f, err := os.Create(fname)
		if err != nil {
			panic(err)
		}
		_, err = io.Copy(f, s)
		if err != nil {
			panic(err)
		}
		cleanup.DumpAndCloseStream(f)
		cleanup.DumpAndCloseStream(s)
	}

	logrus.Info("Deleting export now that it has been dumped")
	for _, p := range parts {
		logrus.Info("Finding datastore for ", p.FileName, " / ", p.DatastoreID)
		ds, err := datastore.LocateDatastore(ctx, p.DatastoreID)
		if err != nil {
			panic(err)
		}

		logrus.Info("Deleting object ", p.Location)
		err = ds.DeleteObject(p.Location)
		if err != nil {
			panic(err)
		}
	}

	logrus.Info("Purging export from database")
	err = exportDb.DeleteExportAndParts(exportId)
	if err != nil {
		panic(err)
	}

	logrus.Infof("Export complete! Files for %s should be in %s", *entity, *destination)
}
