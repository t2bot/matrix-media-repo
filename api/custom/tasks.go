package custom

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
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

func GetTask(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	params := mux.Vars(r)

	taskIdStr := params["taskId"]
	taskId, err := strconv.Atoi(taskIdStr)
	if err != nil {
		log.Error(err)
		return api.BadRequest("invalid task ID")
	}

	log = log.WithFields(logrus.Fields{
		"taskId": taskId,
	})

	db := storage.GetDatabase().GetMetadataStore(r.Context(), log)

	task, err := db.GetBackgroundTask(taskId)
	if err != nil {
		log.Error(err)
		return api.InternalServerError("failed to get task information")
	}

	return &api.DoNotCacheResponse{Payload: &TaskStatus{
		TaskID:     task.ID,
		Name:       task.Name,
		Params:     task.Params,
		StartTs:    task.StartTs,
		EndTs:      task.EndTs,
		IsFinished: task.EndTs > 0,
	}}
}

func ListAllTasks(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	db := storage.GetDatabase().GetMetadataStore(r.Context(), log)

	tasks, err := db.GetAllBackgroundTasks()
	if err != nil {
		logrus.Error(err)
		return api.InternalServerError("Failed to get background tasks")
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

	return &api.DoNotCacheResponse{Payload: statusObjs}
}

func ListUnfinishedTasks(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	db := storage.GetDatabase().GetMetadataStore(r.Context(), log)

	tasks, err := db.GetAllBackgroundTasks()
	if err != nil {
		logrus.Error(err)
		return api.InternalServerError("Failed to get background tasks")
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

	return &api.DoNotCacheResponse{Payload: statusObjs}
}
