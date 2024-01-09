package task_runner

import (
	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/util"
)

func markDone(ctx rcontext.RequestContext, task *database.DbTask) {
	taskDb := database.GetInstance().Tasks.Prepare(ctx)
	if err := taskDb.SetEndTime(task.TaskId, util.NowMillis()); err != nil {
		ctx.Log.Warn("Error updating task as complete: ", err)
		sentry.CaptureException(err)
	}
	ctx.Log.Infof("Task '%s' completed", task.Name)
}

func markError(ctx rcontext.RequestContext, task *database.DbTask, errVal error) {
	taskDb := database.GetInstance().Tasks.Prepare(ctx)
	if err := taskDb.SetError(task.TaskId, errVal.Error()); err != nil {
		ctx.Log.Warn("Error updating task with error message: ", err)
		sentry.CaptureException(err)
	}
	ctx.Log.Debugf("Task '%s' flagged with error", task.Name)
}
