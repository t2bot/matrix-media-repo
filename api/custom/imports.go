package custom

import (
	"errors"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/api/_routers"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/tasks"
	"github.com/t2bot/matrix-media-repo/tasks/task_runner"
	"github.com/t2bot/matrix-media-repo/util/ids"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

type ImportStarted struct {
	ImportID string `json:"import_id"`
	TaskID   int    `json:"task_id"`
}

func StartImport(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	if !rctx.Config.Archiving.Enabled {
		return _responses.BadRequest("archiving is not enabled")
	}
	if ids.GetMachineId() != tasks.ExecutingMachineId {
		return _responses.BadRequest("archival import can only be done on the background tasks worker")
	}

	task, importId, err := tasks.RunImport(rctx)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("fatal error starting import")
	}

	err = task_runner.AppendImportFile(rctx, importId, r.Body)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("error appending first file to import")
	}

	return &_responses.DoNotCacheResponse{Payload: &ImportStarted{
		TaskID:   task.TaskId,
		ImportID: importId,
	}}
}

func AppendToImport(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	if !rctx.Config.Archiving.Enabled {
		return _responses.BadRequest("archiving is not enabled")
	}
	if ids.GetMachineId() != tasks.ExecutingMachineId {
		return _responses.BadRequest("archival import can only be done on the background tasks worker")
	}

	importId := _routers.GetParam("importId", r)

	if !_routers.ServerNameRegex.MatchString(importId) {
		return _responses.BadRequest("invalid import ID")
	}

	err := task_runner.AppendImportFile(rctx, importId, r.Body)
	if err != nil {
		if errors.Is(err, common.ErrMediaNotFound) {
			return _responses.NotFoundError()
		}
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("error appending to import")
	}

	return &_responses.DoNotCacheResponse{Payload: &_responses.EmptyResponse{}}
}

func StopImport(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	if !rctx.Config.Archiving.Enabled {
		return _responses.BadRequest("archiving is not enabled")
	}
	if ids.GetMachineId() != tasks.ExecutingMachineId {
		return _responses.BadRequest("archival import can only be done on the background tasks worker")
	}

	importId := _routers.GetParam("importId", r)

	if !_routers.ServerNameRegex.MatchString(importId) {
		return _responses.BadRequest("invalid import ID")
	}

	err := task_runner.FinishImport(rctx, importId)
	if err != nil {
		if errors.Is(err, common.ErrMediaNotFound) {
			return _responses.NotFoundError()
		}
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("error stopping import")
	}

	return &_responses.DoNotCacheResponse{Payload: &_responses.EmptyResponse{}}
}
