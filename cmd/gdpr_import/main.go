package main

import (
	"flag"
	"io/ioutil"
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
	"github.com/turt2live/matrix-media-repo/util"
)

func main() {
	configPath := flag.String("config", "media-repo.yaml", "The path to the configuration")
	migrationsPath := flag.String("migrations", config.DefaultMigrationsPath, "The absolute path for the migrations folder")
	filesDir := flag.String("directory", "./gdpr-data", "The directory for where the entity's exported files are")
	verifyMode := flag.Bool("verify", false, "If set, no media will be imported and instead be tested to see if they've been imported already")
	flag.Parse()

	// Override config path with config for Docker users
	configEnv := os.Getenv("REPO_CONFIG")
	if configEnv != "" {
		configPath = &configEnv
	}

	config.Path = *configPath
	assets.SetupMigrations(*migrationsPath)

	var err error
	err = logging.Setup(
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

	logrus.Info("Discovering files...")
	fileInfos, err := ioutil.ReadDir(*filesDir)
	if err != nil {
		panic(err)
	}
	files := make([]string, 0)
	for _, f := range fileInfos {
		files = append(files, path.Join(*filesDir, f.Name()))
	}

	// Find the manifest so we can import as soon as possible
	manifestIdx := 0
	for i, fname := range files {
		logrus.Infof("Checking %s for export manifest", fname)
		f, err := os.Open(fname)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		names, err := data_controller.GetFileNames(f)
		if err != nil {
			panic(err)
		}

		if util.ArrayContains(names, "manifest.json") {
			manifestIdx = i
			break
		}
	}

	ctx := rcontext.Initial().LogWithFields(logrus.Fields{"flagDir": *filesDir})

	f, err := os.Open(files[manifestIdx])
	if err != nil {
		panic(err)
	}
	defer f.Close()

	if *verifyMode {
		found, expected, missingIds, err := data_controller.VerifyImport(f, ctx)
		if err != nil {
			panic(err)
		}
		logrus.Info("Known imported media IDs: ", found)
		logrus.Info("Expected media IDs: ", expected)

		if len(missingIds) > 0 {
			for _, mxc := range missingIds {
				logrus.Error("Expected media ID but was not present: ", mxc)
			}
			logrus.Warn("Not all media is present. See logs for details.")
			os.Exit(1)
		}
		logrus.Info("All media present!")
		return // exit 0
	}

	logrus.Info("Starting import...")
	task, importId, err := data_controller.StartImport(f, ctx)
	if err != nil {
		panic(err)
	}

	logrus.Info("Appending all other files to import")
	for i, fname := range files {
		if i == manifestIdx {
			continue // already imported
		}

		logrus.Info("Appending ", fname)

		if !data_controller.IsImportWaiting(importId) {
			logrus.Info("Import claimed closed - ignoring file")
			continue
		}

		f, err := os.Open(fname)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		ch, err := data_controller.AppendToImport(importId, f, true)
		if err != nil {
			panic(err)
		}

		if ch == nil {
			logrus.Info("No channel returned by data controller - moving on to next file")
			continue
		}

		logrus.Info("Waiting for file to be processed before moving on")
		<-ch
		close(ch)
	}

	logrus.Info("Waiting for import to complete")
	waitChan := make(chan bool)
	defer close(waitChan)
	go func() {
		// Initial sleep to let the caches fill
		time.Sleep(1 * time.Second)

		ctx := rcontext.Initial().LogWithFields(logrus.Fields{"async": true})
		db := storage.GetDatabase().GetMetadataStore(ctx)
		for true {
			ctx.Log.Info("Checking if task is complete")

			task, err := db.GetBackgroundTask(task.ID)
			if err != nil {
				logrus.Error(err)
			} else if task.EndTs > 0 {
				waitChan <- true
				return
			}

			time.Sleep(1 * time.Second)
		}
	}()
	<-waitChan

	logrus.Infof("Import complete!")
}
