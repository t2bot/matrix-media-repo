package custom

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/api/_routers"
	"github.com/t2bot/matrix-media-repo/api/apimeta"
	"github.com/t2bot/matrix-media-repo/api/responses"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/datastores"
	"github.com/t2bot/matrix-media-repo/tasks"

	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

type DatastoreMigration struct {
	*datastores.SizeEstimate
	TaskID int `json:"task_id"`
}

func GetDatastores(r *http.Request, rctx rcontext.RequestContext, user apimeta.UserInfo) interface{} {
	response := make(map[string]interface{})
	for _, store := range config.UniqueDatastores() {
		uri, err := datastores.GetUri(store)
		if err != nil {
			sentry.CaptureException(err)
			rctx.Log.Error("Error getting datastore URI: ", err)
			return responses.InternalServerError(errors.New("unexpected error getting datastore information"))
		}
		dataStoreMap := make(map[string]interface{})
		dataStoreMap["type"] = store.Type
		dataStoreMap["uri"] = uri
		response[store.Id] = dataStoreMap
	}

	return &responses.DoNotCacheResponse{Payload: response}
}

func MigrateBetweenDatastores(r *http.Request, rctx rcontext.RequestContext, user apimeta.UserInfo) interface{} {
	beforeTsStr := r.URL.Query().Get("before_ts")
	beforeTs := time.Now().UnixMilli()
	var err error
	if beforeTsStr != "" {
		beforeTs, err = strconv.ParseInt(beforeTsStr, 10, 64)
		if err != nil {
			return responses.BadRequest(fmt.Errorf("Error parsing before_ts: %w", err))
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
		return responses.BadRequest(errors.New("Source and target datastore cannot be the same"))
	}
	if _, ok := datastores.Get(rctx, sourceDsId); !ok {
		return responses.BadRequest(errors.New("Source datastore does not appear to exist"))
	}
	if _, ok := datastores.Get(rctx, targetDsId); !ok {
		return responses.BadRequest(errors.New("Target datastore does not appear to exist"))
	}

	estimate, err := datastores.SizeOfDsIdWithAge(rctx, sourceDsId, beforeTs)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return responses.InternalServerError(errors.New("Unexpected error getting storage estimate"))
	}

	rctx.Log.Infof("User %s has started a datastore media transfer", user.UserId)
	task, err := tasks.RunDatastoreMigration(rctx, sourceDsId, targetDsId, beforeTs)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return responses.InternalServerError(errors.New("Unexpected error starting migration"))
	}

	migration := &DatastoreMigration{
		SizeEstimate: estimate,
		TaskID:       task.TaskId,
	}

	return &responses.DoNotCacheResponse{Payload: migration}
}

func GetDatastoreStorageEstimate(r *http.Request, rctx rcontext.RequestContext, user apimeta.UserInfo) interface{} {
	beforeTsStr := r.URL.Query().Get("before_ts")
	beforeTs := time.Now().UnixMilli()
	var err error
	if beforeTsStr != "" {
		beforeTs, err = strconv.ParseInt(beforeTsStr, 10, 64)
		if err != nil {
			return responses.BadRequest(fmt.Errorf("Error parsing before_ts: %w", err))
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
		return responses.InternalServerError(errors.New("Unexpected error getting storage estimate"))
	}
	return &responses.DoNotCacheResponse{Payload: result}
}
