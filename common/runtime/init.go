package runtime

import (
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/common/version"
	"github.com/turt2live/matrix-media-repo/ipfs_proxy"
	"github.com/turt2live/matrix-media-repo/plugins"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/storage/datastore/ds_s3"
)

func RunStartupSequence() {
	version.Print(true)
	config.PrintDomainInfo()
	config.CheckDeprecations()
	LoadDatabase()
	LoadDatastores()
	plugins.ReloadPlugins()

	logrus.Info("Starting IPFS (if enabled)...")
	ipfs_proxy.Reload()
}

func LoadDatabase() {
	logrus.Info("Preparing database...")
	storage.GetDatabase()
}

func LoadDatastores() {
	mediaStore := storage.GetDatabase().GetMediaStore(rcontext.Initial())

	logrus.Info("Initializing datastores...")
	for _, ds := range config.UniqueDatastores() {
		if !ds.Enabled {
			continue
		}

		uri := datastore.GetUriForDatastore(ds)

		_, err := storage.GetOrCreateDatastoreOfType(rcontext.Initial(), ds.Type, uri)
		if err != nil {
			sentry.CaptureException(err)
			logrus.Fatal(err)
		}
	}

	// Print all the known datastores at startup. Doubles as a way to initialize the database.
	datastores, err := mediaStore.GetAllDatastores()
	if err != nil {
		sentry.CaptureException(err)
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
				logrus.Warn("\t\tBucket does not exist!", err)
			}

			err = s3.EnsureTempPathExists()
			if err != nil {
				logrus.Warn("\t\tTemporary path does not exist!")
			}
		}
	}
}
