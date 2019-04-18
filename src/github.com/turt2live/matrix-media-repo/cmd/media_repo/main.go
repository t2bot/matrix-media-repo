package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api/webserver"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/logging"
	"github.com/turt2live/matrix-media-repo/metrics"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/storage/datastore/ds_s3"
)

func main() {
	configPath := flag.String("config", "media-repo.yaml", "The path to the configuration")
	migrationsPath := flag.String("migrations", "./migrations", "The absolute path the migrations folder")
	flag.Parse()

	config.Path = *configPath
	config.Runtime.MigrationsPath = *migrationsPath

	err := logging.Setup(config.Get().General.LogDirectory)
	if err != nil {
		panic(err)
	}

	if len(config.Get().Uploads.StoragePaths) > 0 {
		logrus.Warn("storagePaths usage is deprecated - please use datastores instead")
		for _, p := range config.Get().Uploads.StoragePaths {
			ds, err := storage.GetOrCreateDatastoreOfType(context.Background(), logrus.WithFields(logrus.Fields{"path": p}), "file", p)
			if err != nil {
				logrus.Fatal(err)
			}

			fakeConfig := config.DatastoreConfig{
				Type:       "file",
				Enabled:    true,
				ForUploads: true,
				Options:    map[string]string{"path": ds.Uri},
			}
			config.Get().DataStores = append(config.Get().DataStores, fakeConfig)
		}
	}

	mediaStore := storage.GetDatabase().GetMediaStore(context.TODO(), &logrus.Entry{})

	logrus.Info("Initializing datastores...")
	enabledDatastores := 0
	for _, ds := range config.Get().DataStores {
		if !ds.Enabled {
			continue
		}

		enabledDatastores++
		uri := datastore.GetUriForDatastore(ds)

		_, err := storage.GetOrCreateDatastoreOfType(context.TODO(), &logrus.Entry{}, ds.Type, uri)
		if err != nil {
			logrus.Fatal(err)
		}
	}

	// Print all the known datastores at startup. Doubles as a way to initialize the database.
	datastores, err := mediaStore.GetAllDatastores()
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Info("Datastores:")
	for _, ds := range datastores {
		logrus.Info(fmt.Sprintf("\t%s (%s): %s", ds.Type, ds.DatastoreId, ds.Uri))

		if ds.Type == "s3" {
			conf, err := datastore.GetDatastoreConfig(ds)
			if err != nil {
				continue
			}

			s3, err := ds_s3.GetOrCreateS3Datastore(ds.DatastoreId, conf)
			if err != nil {
				continue
			}

			err = s3.EnsureBucketExists()
			if err != nil {
				logrus.Warn("\t\tBucket does not exist!")
			}
		}
	}

	if len(config.Get().Uploads.StoragePaths) > 0 {
		logrus.Warn("You are using `storagePaths` in your configuration - in a future update, this will be removed. Please use datastores instead (see sample config).")
	}

	logrus.Info("Starting media repository...")
	metrics.Init()
	webserver.Init() // blocks to listen for requests
}
