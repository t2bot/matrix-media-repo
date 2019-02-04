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

	mediaStore := storage.GetDatabase().GetMediaStore(context.TODO(), &logrus.Entry{})

	logrus.Info("Initializing datastores...")
	enabledDatastores := 0
	for _, ds:=range config.Get().Uploads.DataStores {
		if !ds.Enabled {
			continue
		}

		enabledDatastores++

		uri := ""
		if ds.Type == "file" {
			path, pathFound := ds.Options["path"]
			if !pathFound {
				logrus.Fatal("Missing 'path' on file datastore")
			}
			uri = path
		} else if ds.Type == "s3" {
			endpoint, epFound := ds.Options["endpoint"]
			bucket, bucketFound := ds.Options["bucketName"]
			if !epFound || !bucketFound {
				logrus.Fatal("Missing 'endpoint' or 'bucketName' on s3 datastore")
			}
			uri = fmt.Sprintf("s3://%s/%s", endpoint, bucket)
		} else {
			logrus.Fatal("Unknown datastore type: ", ds.Type)
		}

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
	}

	// TODO: https://github.com/minio/minio-go support

	logrus.Info("Starting media repository...")
	metrics.Init()
	webserver.Init() // blocks to listen for requests
}
