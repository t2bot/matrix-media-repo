package main

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/controllers/maintenance_controller"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/storage/datastore/ds_s3"
)

func scanAndStartUnfinishedTasks() error {
	ctx := context.Background()
	log := logrus.WithFields(logrus.Fields{"stage": "startup"})
	db := storage.GetDatabase().GetMetadataStore(ctx, log)
	tasks, err := db.GetAllBackgroundTasks()
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if task.EndTs > 0 {
			continue
		}

		taskLog := log.WithFields(logrus.Fields{
			"prev_task_id":   task.ID,
			"prev_task_name": task.Name,
		})

		if task.Name == "storage_migration" {
			beforeTs := int64(task.Params["before_ts"].(float64))
			sourceDsId := task.Params["source_datastore_id"].(string)
			targetDsId := task.Params["target_datastore_id"].(string)

			sourceDs, err := datastore.LocateDatastore(ctx, taskLog, sourceDsId)
			if err != nil {
				return err
			}
			targetDs, err := datastore.LocateDatastore(ctx, taskLog, targetDsId)
			if err != nil {
				return err
			}

			newTask, err := maintenance_controller.StartStorageMigration(sourceDs, targetDs, beforeTs, taskLog)
			if err != nil {
				return err
			}

			err = db.FinishedBackgroundTask(task.ID)
			if err != nil {
				return err
			}

			taskLog.Infof("Started replacement task ID %d for unfinished task %d (%s)", newTask.ID, task.ID, task.Name)
		} else {
			taskLog.Warn(fmt.Sprintf("Unknown task %s at ID %d - ignoring", task.Name, task.ID))
		}
	}

	return nil
}

func loadDatabase() {
	logrus.Info("Preparing database...")
	storage.GetDatabase()
}

func loadDatastores() {
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
				MediaKinds: common.AllKinds,
				Options:    map[string]string{"path": ds.Uri},
			}
			config.Get().DataStores = append(config.Get().DataStores, fakeConfig)
		}
	}

	mediaStore := storage.GetDatabase().GetMediaStore(context.TODO(), &logrus.Entry{})

	logrus.Info("Initializing datastores...")
	for _, ds := range config.Get().DataStores {
		if !ds.Enabled {
			continue
		}

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
