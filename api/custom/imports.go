package custom

import (
	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/api/_routers"
	"github.com/turt2live/matrix-media-repo/util/stream_util"

	"net/http"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/data_controller"
)

type ImportStarted struct {
	ImportID string `json:"import_id"`
	TaskID   int    `json:"task_id"`
}

func StartImport(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	if !rctx.Config.Archiving.Enabled {
		return _responses.BadRequest("archiving is not enabled")
	}

	defer stream_util.DumpAndCloseStream(r.Body)
	task, importId, err := data_controller.StartImport(r.Body, rctx)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("fatal error starting import")
	}

	return &_responses.DoNotCacheResponse{Payload: &ImportStarted{
		TaskID:   task.ID,
		ImportID: importId,
	}}
}

func AppendToImport(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	if !rctx.Config.Archiving.Enabled {
		return _responses.BadRequest("archiving is not enabled")
	}

	importId := _routers.GetParam("importId", r)

	if !_routers.ServerNameRegex.MatchString(importId) {
		return _responses.BadRequest("invalid import ID")
	}

	defer stream_util.DumpAndCloseStream(r.Body)
	_, err := data_controller.AppendToImport(importId, r.Body, false)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("fatal error appending to import")
	}

	return &_responses.DoNotCacheResponse{Payload: &_responses.EmptyResponse{}}
}

func StopImport(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	if !rctx.Config.Archiving.Enabled {
		return _responses.BadRequest("archiving is not enabled")
	}

	importId := _routers.GetParam("importId", r)

	if !_routers.ServerNameRegex.MatchString(importId) {
		return _responses.BadRequest("invalid import ID")
	}

	err := data_controller.StopImport(importId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("fatal error stopping import")
	}

	return &_responses.DoNotCacheResponse{Payload: &_responses.EmptyResponse{}}
}
