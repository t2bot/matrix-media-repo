package tasks

import (
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/notifier"
	"github.com/turt2live/matrix-media-repo/tasks/task_runner"
	"github.com/turt2live/matrix-media-repo/util/ids"
)

var notiferCh <-chan notifier.TaskId

func executeEnable() {
	if notiferCh != nil {
		return
	}
	if ids.GetMachineId() == ExecutingMachineId {
		notiferCh = notifier.SubscribeToTasks()
		if notiferCh != nil {
			go func() {
				for val := range notiferCh {
					tryBeginTask(int(val), true)
				}
				notiferCh = nil
			}()
		}
	}
}

func tryBeginTask(id int, recur bool) {
	ctx := rcontext.Initial().LogWithFields(logrus.Fields{"background_task_waiter": "redis"})
	ctx.Log.Debug("Got new task to try running: ", id)
	db := database.GetInstance().Tasks.Prepare(ctx)
	t, err := db.Get(id)
	if err != nil {
		// Dev note: we capture the exception in each branch to identify whether it's an error on retry
		if recur {
			sentry.CaptureException(err)
			ctx.Log.Error("Failed to find published background task - will try once more in 30 seconds")
			go func() {
				time.Sleep(30 * time.Second)
				tryBeginTask(id, false)
			}()
		} else {
			sentry.CaptureException(err)
			ctx.Log.Error("Failed to find published background task after retry - giving up")
		}
		return
	}
	beginTask(t)
}

func beginTask(task *database.DbTask) {
	if task.EndTs > 0 {
		return // just skip it
	}
	// TODO: Worker group: https://github.com/turt2live/matrix-media-repo/issues/425
	runnerCtx := rcontext.Initial().LogWithFields(logrus.Fields{"task_id": task.TaskId})
	if task.Name == string(TaskDatastoreMigrate) {
		go task_runner.DatastoreMigrate(runnerCtx, task)
	} else if task.Name == string(TaskExportData) {
		go task_runner.ExportData(runnerCtx, task)
	} else if task.Name == string(TaskImportData) {
		go task_runner.ImportData(runnerCtx, task)
	} else {
		m := fmt.Sprintf("Received unknown task to run %s (ID: %d)", task.Name, task.TaskId)
		logrus.Warn(m)
		sentry.CaptureMessage(m)
	}
}
