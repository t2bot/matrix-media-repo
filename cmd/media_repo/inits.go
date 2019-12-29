package main

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/maintenance_controller"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
)

func scanAndStartUnfinishedTasks() error {
	ctx := rcontext.Initial().LogWithFields(logrus.Fields{"stage": "startup"})
	db := storage.GetDatabase().GetMetadataStore(ctx)
	tasks, err := db.GetAllBackgroundTasks()
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if task.EndTs > 0 {
			continue
		}

		taskCtx := ctx.LogWithFields(logrus.Fields{
			"prev_task_id":   task.ID,
			"prev_task_name": task.Name,
		})

		if task.Name == "storage_migration" {
			beforeTs := int64(task.Params["before_ts"].(float64))
			sourceDsId := task.Params["source_datastore_id"].(string)
			targetDsId := task.Params["target_datastore_id"].(string)

			sourceDs, err := datastore.LocateDatastore(taskCtx, sourceDsId)
			if err != nil {
				return err
			}
			targetDs, err := datastore.LocateDatastore(taskCtx, targetDsId)
			if err != nil {
				return err
			}

			newTask, err := maintenance_controller.StartStorageMigration(sourceDs, targetDs, beforeTs, taskCtx)
			if err != nil {
				return err
			}

			err = db.FinishedBackgroundTask(task.ID)
			if err != nil {
				return err
			}

			taskCtx.Log.Infof("Started replacement task ID %d for unfinished task %d (%s)", newTask.ID, task.ID, task.Name)
		} else {
			taskCtx.Log.Warn(fmt.Sprintf("Unknown task %s at ID %d - ignoring", task.Name, task.ID))
		}
	}

	return nil
}

