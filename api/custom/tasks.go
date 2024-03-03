package custom

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/api/_routers"
	"github.com/t2bot/matrix-media-repo/api/apimeta"
	"github.com/t2bot/matrix-media-repo/api/responses"
	"github.com/t2bot/matrix-media-repo/database"

	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

type TaskStatus struct {
	TaskID     int                     `json:"task_id"`
	Name       string                  `json:"task_name"`
	Params     *database.AnonymousJson `json:"params"`
	StartTs    int64                   `json:"start_ts"`
	EndTs      int64                   `json:"end_ts"`
	IsFinished bool                    `json:"is_finished"`
	Error      string                  `json:"error_message"`
}

func GetTask(r *http.Request, rctx rcontext.RequestContext, user apimeta.UserInfo) interface{} {
	taskIdStr := _routers.GetParam("taskId", r)
	taskId, err := strconv.Atoi(taskIdStr)
	if err != nil {
		rctx.Log.Error(err)
		return responses.BadRequest(errors.New("invalid task ID"))
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"taskId": taskId,
	})

	db := database.GetInstance().Tasks.Prepare(rctx)

	task, err := db.Get(taskId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return responses.InternalServerError(errors.New("failed to get task information"))
	}
	if task == nil {
		return responses.NotFoundError()
	}

	return &responses.DoNotCacheResponse{Payload: &TaskStatus{
		TaskID:     task.TaskId,
		Name:       task.Name,
		Params:     task.Params,
		StartTs:    task.StartTs,
		EndTs:      task.EndTs,
		IsFinished: task.EndTs > 0,
		Error:      task.Error,
	}}
}

func ListAllTasks(r *http.Request, rctx rcontext.RequestContext, user apimeta.UserInfo) interface{} {
	db := database.GetInstance().Tasks.Prepare(rctx)

	tasks, err := db.GetAll(true)
	if err != nil {
		logrus.Error(err)
		sentry.CaptureException(err)
		return responses.InternalServerError(errors.New("Failed to get background tasks"))
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
			Error:      task.Error,
		})
	}

	return &responses.DoNotCacheResponse{Payload: statusObjs}
}

func ListUnfinishedTasks(r *http.Request, rctx rcontext.RequestContext, user apimeta.UserInfo) interface{} {
	db := database.GetInstance().Tasks.Prepare(rctx)

	tasks, err := db.GetAll(false)
	if err != nil {
		logrus.Error(err)
		sentry.CaptureException(err)
		return responses.InternalServerError(errors.New("Failed to get background tasks"))
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
			Error:      task.Error,
		})
	}

	return &responses.DoNotCacheResponse{Payload: statusObjs}
}
