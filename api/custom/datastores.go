package custom

import (
	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/api/_routers"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/datastores"

	"net/http"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/maintenance_controller"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type DatastoreMigration struct {
	*types.DatastoreMigrationEstimate
	TaskID int `json:"task_id"`
}

func GetDatastores(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	response := make(map[string]interface{})
	for _, ds := range config.UniqueDatastores() {
		uri, err := datastores.GetUri(ds)
		if err != nil {
			sentry.CaptureException(err)
			rctx.Log.Error("Error getting datastore URI: ", err)
			return _responses.InternalServerError("unexpected error getting datastore information")
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
			return _responses.BadRequest("Error parsing before_ts: " + err.Error())
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
		return _responses.BadRequest("Source and target datastore cannot be the same")
	}

	sourceDatastore, err := datastore.LocateDatastore(rctx, sourceDsId)
	if err != nil {
		rctx.Log.Error(err)
		return _responses.BadRequest("Error getting source datastore. Does it exist?")
	}

	targetDatastore, err := datastore.LocateDatastore(rctx, targetDsId)
	if err != nil {
		rctx.Log.Error(err)
		return _responses.BadRequest("Error getting target datastore. Does it exist?")
	}

	rctx.Log.Info("User ", user.UserId, " has started a datastore media transfer")
	task, err := maintenance_controller.StartStorageMigration(sourceDatastore, targetDatastore, beforeTs, rctx)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected error starting migration")
	}

	estimate, err := maintenance_controller.EstimateDatastoreSizeWithAge(beforeTs, sourceDsId, rctx)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected error getting storage estimate")
	}

	migration := &DatastoreMigration{
		DatastoreMigrationEstimate: estimate,
		TaskID:                     task.ID,
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
			return _responses.BadRequest("Error parsing before_ts: " + err.Error())
		}
	}

	datastoreId := _routers.GetParam("datastoreId", r)

	rctx = rctx.LogWithFields(logrus.Fields{
		"beforeTs":    beforeTs,
		"datastoreId": datastoreId,
	})

	result, err := maintenance_controller.EstimateDatastoreSizeWithAge(beforeTs, datastoreId, rctx)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected error getting storage estimate")
	}
	return &_responses.DoNotCacheResponse{Payload: result}
}
