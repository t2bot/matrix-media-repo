package custom

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/api/_routers"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/datastores"
	"github.com/t2bot/matrix-media-repo/tasks"

	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/util"
)

type DatastoreMigration struct {
	*datastores.SizeEstimate
	TaskID int `json:"task_id"`
}

func GetDatastores(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	response := make(map[string]interface{})
	for _, ds := range config.UniqueDatastores() {
		uri, err := datastores.GetUri(ds)
		if err != nil {
			sentry.CaptureException(err)
			rctx.Log.Error("Error getting datastore URI: ", err)
			return _responses.InternalServerError(errors.New("unexpected error getting datastore information"))
		}
		dsMap := make(map[string]interface{})
		dsMap["type"] = ds.Type
		dsMap["uri"] = uri
		response[ds.Id] = dsMap
	}

	return &_responses.DoNotCacheResponse{Payload: response}
}

func MigrateBetweenDatastores(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	beforeTsStr := r.URL.Query().Get("before_ts")
	beforeTs := util.NowMillis()
	var err error
	if beforeTsStr != "" {
		beforeTs, err = strconv.ParseInt(beforeTsStr, 10, 64)
		if err != nil {
			return _responses.BadRequest(fmt.Errorf("Error parsing before_ts: %w", err))
		}
	}

	sourceDsId := _routers.GetParam("sourceDsId", r)
	targetDsId := _routers.GetParam("targetDsId", r)

	rctx = rctx.LogWithFields(logrus.Fields{
		"beforeTs":   beforeTs,
		"sourceDsId": sourceDsId,
		"targetDsId": targetDsId,
	})

	if sourceDsId == targetDsId {
		return _responses.BadRequest(errors.New("Source and target datastore cannot be the same"))
	}
	if _, ok := datastores.Get(rctx, sourceDsId); !ok {
		return _responses.BadRequest(errors.New("Source datastore does not appear to exist"))
	}
	if _, ok := datastores.Get(rctx, targetDsId); !ok {
		return _responses.BadRequest(errors.New("Target datastore does not appear to exist"))
	}

	estimate, err := datastores.SizeOfDsIdWithAge(rctx, sourceDsId, beforeTs)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(errors.New("Unexpected error getting storage estimate"))
	}

	rctx.Log.Infof("User %s has started a datastore media transfer", user.UserId)
	task, err := tasks.RunDatastoreMigration(rctx, sourceDsId, targetDsId, beforeTs)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(errors.New("Unexpected error starting migration"))
	}

	migration := &DatastoreMigration{
		SizeEstimate: estimate,
		TaskID:       task.TaskId,
	}

	return &_responses.DoNotCacheResponse{Payload: migration}
}

func GetDatastoreStorageEstimate(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	beforeTsStr := r.URL.Query().Get("before_ts")
	beforeTs := util.NowMillis()
	var err error
	if beforeTsStr != "" {
		beforeTs, err = strconv.ParseInt(beforeTsStr, 10, 64)
		if err != nil {
			return _responses.BadRequest(fmt.Errorf("Error parsing before_ts: %w", err))
		}
	}

	datastoreId := _routers.GetParam("datastoreId", r)

	rctx = rctx.LogWithFields(logrus.Fields{
		"beforeTs":    beforeTs,
		"datastoreId": datastoreId,
	})

	result, err := datastores.SizeOfDsIdWithAge(rctx, datastoreId, beforeTs)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(errors.New("Unexpected error getting storage estimate"))
	}
	return &_responses.DoNotCacheResponse{Payload: result}
}
