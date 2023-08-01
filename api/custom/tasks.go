package custom

import (
	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/api/_routers"
	"github.com/turt2live/matrix-media-repo/database"

	"net/http"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

type TaskStatus struct {
	TaskID     int                     `json:"task_id"`
	Name       string                  `json:"task_name"`
	Params     *database.AnonymousJson `json:"params"`
	StartTs    int64                   `json:"start_ts"`
	EndTs      int64                   `json:"end_ts"`
	IsFinished bool                    `json:"is_finished"`
}

func GetTask(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	taskIdStr := _routers.GetParam("taskId", r)
	taskId, err := strconv.Atoi(taskIdStr)
	if err != nil {
		rctx.Log.Error(err)
		return _responses.BadRequest("invalid task ID")
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"taskId": taskId,
	})

	db := database.GetInstance().Tasks.Prepare(rctx)

	task, err := db.Get(taskId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to get task information")
	}
	if task == nil {
		return _responses.NotFoundError()
	}

	return &_responses.DoNotCacheResponse{Payload: &TaskStatus{
		TaskID:     task.TaskId,
		Name:       task.Name,
		Params:     task.Params,
		StartTs:    task.StartTs,
		EndTs:      task.EndTs,
		IsFinished: task.EndTs > 0,
	}}
}

func ListAllTasks(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	db := database.GetInstance().Tasks.Prepare(rctx)

	tasks, err := db.GetAll(true)
	if err != nil {
		logrus.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Failed to get background tasks")
	}

	statusObjs := make([]*TaskStatus, 0)
	for _, task := range tasks {
		statusObjs = append(statusObjs, &TaskStatus{
			TaskID:     task.TaskId,
			Name:       task.Name,
			Params:     task.Params,
			StartTs:    task.StartTs,
			EndTs:      task.EndTs,
			IsFinished: task.EndTs > 0,
		})
	}

	return &_responses.DoNotCacheResponse{Payload: statusObjs}
}

func ListUnfinishedTasks(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	db := database.GetInstance().Tasks.Prepare(rctx)

	tasks, err := db.GetAll(false)
	if err != nil {
		logrus.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Failed to get background tasks")
	}

	statusObjs := make([]*TaskStatus, 0)
	for _, task := range tasks {
		statusObjs = append(statusObjs, &TaskStatus{
			TaskID:     task.TaskId,
			Name:       task.Name,
			Params:     task.Params,
			StartTs:    task.StartTs,
			EndTs:      task.EndTs,
			IsFinished: task.EndTs > 0,
		})
	}

	return &_responses.DoNotCacheResponse{Payload: statusObjs}
}
