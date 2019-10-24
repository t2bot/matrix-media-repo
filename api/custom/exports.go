package custom

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/controllers/data_controller"
	"github.com/turt2live/matrix-media-repo/util"
)

type ExportStarted struct {
	ExportID string `json:"export_id"`
	TaskID   int    `json:"task_id"`
}

func ExportUserData(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	if !config.Get().Archiving.Enabled {
		return api.BadRequest("archiving is not enabled")
	}

	isAdmin := util.IsGlobalAdmin(user.UserId) || user.IsShared
	if !config.Get().Archiving.SelfService && !isAdmin {
		return api.AuthFailed()
	}

	includeData := r.URL.Query().Get("include_data") != "false"
	s3urls := r.URL.Query().Get("s3_urls") != "false"

	params := mux.Vars(r)

	userId := params["userId"]

	if !isAdmin && user.UserId != userId {
		return api.BadRequest("cannot export data for another user")
	}

	log = log.WithFields(logrus.Fields{
		"exportUserId": userId,
		"includeData":  includeData,
		"s3urls":       s3urls,
	})
	task, exportId, err := data_controller.StartUserExport(userId, s3urls, includeData, log)
	if err != nil {
		log.Error(err)
		return api.InternalServerError("fatal error starting export")
	}

	return &api.DoNotCacheResponse{Payload: &ExportStarted{
		TaskID:   task.ID,
		ExportID: exportId,
	}}
}

func ViewExport(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	if !config.Get().Archiving.Enabled {
		return api.BadRequest("archiving is not enabled")
	}

	return api.EmptyResponse{}
}

func GetExportMetadata(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	if !config.Get().Archiving.Enabled {
		return api.BadRequest("archiving is not enabled")
	}

	return api.EmptyResponse{}
}

func DownloadExportPart(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	if !config.Get().Archiving.Enabled {
		return api.BadRequest("archiving is not enabled")
	}

	return api.EmptyResponse{}
}

func DeleteExport(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	if !config.Get().Archiving.Enabled {
		return api.BadRequest("archiving is not enabled")
	}

	return api.EmptyResponse{}
}
