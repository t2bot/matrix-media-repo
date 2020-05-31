package runtime

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/common/version"
	"github.com/turt2live/matrix-media-repo/ipfs_proxy"
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

	logrus.Info("Starting IPFS (if enabled)...")
	ipfs_proxy.Reload()
}

func LoadDatabase() {
	logrus.Info("Preparing database...")
	storage.GetDatabase()
}

func LoadDatastores() {
	if len(config.Get().Uploads.StoragePaths) > 0 {
		logrus.Warn("storagePaths usage is deprecated - please use datastores instead")
		for _, p := range config.Get().Uploads.StoragePaths {
			ctx := rcontext.Initial().LogWithFields(logrus.Fields{"path": p})
			ds, err := storage.GetOrCreateDatastoreOfType(ctx, "file", p)
			if err != nil {
				logrus.Fatal(err)
			}

			fakeConfig := config.DatastoreConfig{
				Type:       "file",
				Enabled:    true,
				MediaKinds: common.AllKinds,
				Options:    map[string]string{"path": ds.Uri},
			}
			config.Get().DataStores = append(config.Get().DataStores, fakeConfig)
		}
	}

	mediaStore := storage.GetDatabase().GetMediaStore(rcontext.Initial())

	logrus.Info("Initializing datastores...")
	for _, ds := range config.UniqueDatastores() {
		if !ds.Enabled {
			continue
		}

		uri := datastore.GetUriForDatastore(ds)

		_, err := storage.GetOrCreateDatastoreOfType(rcontext.Initial(), ds.Type, uri)
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

			err = s3.EnsureTempPathExists()
			if err != nil {
				logrus.Warn("\t\tTemporary path does not exist!")
			}
		}
	}

	if len(config.Get().Uploads.StoragePaths) > 0 {
		logrus.Warn("You are using `storagePaths` in your configuration - in a future update, this will be removed. Please use datastores instead (see sample config).")
	}
}
