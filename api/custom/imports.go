package custom

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/data_controller"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
)

type ImportStarted struct {
	ImportID string `json:"import_id"`
	TaskID   int    `json:"task_id"`
}

func StartImport(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	if !rctx.Config.Archiving.Enabled {
		return api.BadRequest("archiving is not enabled")
	}

	defer cleanup.DumpAndCloseStream(r.Body)
	task, importId, err := data_controller.StartImport(r.Body, rctx)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("fatal error starting import")
	}

	return &api.DoNotCacheResponse{Payload: &ImportStarted{
		TaskID:   task.ID,
		ImportID: importId,
	}}
}

func AppendToImport(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	if !rctx.Config.Archiving.Enabled {
		return api.BadRequest("archiving is not enabled")
	}

	params := mux.Vars(r)

	importId := params["importId"]

	defer cleanup.DumpAndCloseStream(r.Body)
	_, err := data_controller.AppendToImport(importId, r.Body, false)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("fatal error appending to import")
	}

	return &api.DoNotCacheResponse{Payload: &api.EmptyResponse{}}
}

func StopImport(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	if !rctx.Config.Archiving.Enabled {
		return api.BadRequest("archiving is not enabled")
	}

	params := mux.Vars(r)

	importId := params["importId"]

	err := data_controller.StopImport(importId)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("fatal error stopping import")
	}

	return &api.DoNotCacheResponse{Payload: &api.EmptyResponse{}}
}
