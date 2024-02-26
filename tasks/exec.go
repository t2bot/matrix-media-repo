package tasks

import (
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/notifier"
	"github.com/t2bot/matrix-media-repo/pool"
	"github.com/t2bot/matrix-media-repo/tasks/task_runner"
	"github.com/t2bot/matrix-media-repo/util"
	"github.com/t2bot/matrix-media-repo/util/ids"
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
					go tryBeginTask(int(val), true)
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
	runnerCtx := rcontext.Initial().LogWithFields(logrus.Fields{"task_id": task.TaskId})

	oneHourAgo := util.NowMillis() - (60 * 60 * 1000)
	if task.StartTs < oneHourAgo {
		runnerCtx.Log.Warn("Not starting task because it is more than 1 hour old.")
		return
	}

	if err := pool.TaskQueue.Schedule(func() {
		if task.Name == string(TaskDatastoreMigrate) {
			task_runner.DatastoreMigrate(runnerCtx, task)
		} else if task.Name == string(TaskExportData) {
			task_runner.ExportData(runnerCtx, task)
		} else if task.Name == string(TaskImportData) {
			task_runner.ImportData(runnerCtx, task)
		} else {
			m := fmt.Sprintf("Received unknown task to run %s (ID: %d)", task.Name, task.TaskId)
			runnerCtx.Log.Warn(m)
			sentry.CaptureMessage(m)
		}
	}); err != nil {
		m := fmt.Sprintf("Error trying to schedule task %s (ID: %d): %v", task.Name, task.TaskId, err)
		runnerCtx.Log.Warn(m)
		sentry.CaptureMessage(m)
		time.AfterFunc(15*time.Second, func() {
			beginTask(task)
		})
	}
}
