package custom

import (
	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/api/_routers"

	"net/http"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/storage"
)

type TaskStatus struct {
	TaskID     int                    `json:"task_id"`
	Name       string                 `json:"task_name"`
	Params     map[string]interface{} `json:"params"`
	StartTs    int64                  `json:"start_ts"`
	EndTs      int64                  `json:"end_ts"`
	IsFinished bool                   `json:"is_finished"`
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

	db := storage.GetDatabase().GetMetadataStore(rctx)

	task, err := db.GetBackgroundTask(taskId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to get task information")
	}

	return &_responses.DoNotCacheResponse{Payload: &TaskStatus{
		TaskID:     task.ID,
		Name:       task.Name,
		Params:     task.Params,
		StartTs:    task.StartTs,
		EndTs:      task.EndTs,
		IsFinished: task.EndTs > 0,
	}}
}

func ListAllTasks(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	db := storage.GetDatabase().GetMetadataStore(rctx)

	tasks, err := db.GetAllBackgroundTasks()
	if err != nil {
		logrus.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Failed to get background tasks")
	}

	statusObjs := make([]*TaskStatus, 0)
	for _, task := range tasks {
		statusObjs = append(statusObjs, &TaskStatus{
			TaskID:     task.ID,
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
	db := storage.GetDatabase().GetMetadataStore(rctx)

	tasks, err := db.GetAllBackgroundTasks()
	if err != nil {
		logrus.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Failed to get background tasks")
	}

	statusObjs := make([]*TaskStatus, 0)
	for _, task := range tasks {
		if task.EndTs > 0 {
			continue
		}
		statusObjs = append(statusObjs, &TaskStatus{
			TaskID:     task.ID,
			Name:       task.Name,
			Params:     task.Params,
			StartTs:    task.StartTs,
			EndTs:      task.EndTs,
			IsFinished: task.EndTs > 0,
		})
	}

	return &_responses.DoNotCacheResponse{Payload: statusObjs}
}
