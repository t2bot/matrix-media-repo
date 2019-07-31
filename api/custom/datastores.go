package custom

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/controllers/maintenance_controller"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/util"
)

func GetDatastores(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	datastores, err := storage.GetDatabase().GetMediaStore(r.Context(), log).GetAllDatastores()
	if err != nil {
		log.Error(err)
		return api.InternalServerError("Error getting datastores")
	}

	response := make(map[string]interface{})

	for _, ds := range datastores {
		dsMap := make(map[string]interface{})
		dsMap["type"] = ds.Type
		dsMap["uri"] = ds.Uri
		response[ds.DatastoreId] = dsMap
	}

	return &api.DoNotCacheResponse{Payload: response}
}

func MigrateBetweenDatastores(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	beforeTsStr := r.URL.Query().Get("before_ts")
	beforeTs := util.NowMillis()
	var err error
	if beforeTsStr != "" {
		beforeTs, err = strconv.ParseInt(beforeTsStr, 10, 64)
		if err != nil {
			return api.BadRequest("Error parsing before_ts: " + err.Error())
		}
	}

	params := mux.Vars(r)

	sourceDsId := params["sourceDsId"]
	targetDsId := params["targetDsId"]

	log = log.WithFields(logrus.Fields{
		"beforeTs":   beforeTs,
		"sourceDsId": sourceDsId,
		"targetDsId": targetDsId,
	})

	if sourceDsId == targetDsId {
		return api.BadRequest("Source and target datastore cannot be the same")
	}

	sourceDatastore, err := datastore.LocateDatastore(r.Context(), log, sourceDsId)
	if err != nil {
		log.Error(err)
		return api.BadRequest("Error getting source datastore. Does it exist?")
	}

	targetDatastore, err := datastore.LocateDatastore(r.Context(), log, targetDsId)
	if err != nil {
		log.Error(err)
		return api.BadRequest("Error getting target datastore. Does it exist?")
	}

	log.Info("User ", user.UserId, " has started a datastore media transfer")
	maintenance_controller.StartStorageMigration(sourceDatastore, targetDatastore, beforeTs, log)

	estimate, err := maintenance_controller.EstimateDatastoreSizeWithAge(beforeTs, sourceDsId, r.Context(), log)
	if err != nil {
		log.Error(err)
		return api.InternalServerError("Unexpected error getting storage estimate")
	}

	return &api.DoNotCacheResponse{Payload: estimate}
}

func GetDatastoreStorageEstimate(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	beforeTsStr := r.URL.Query().Get("before_ts")
	beforeTs := util.NowMillis()
	var err error
	if beforeTsStr != "" {
		beforeTs, err = strconv.ParseInt(beforeTsStr, 10, 64)
		if err != nil {
			return api.BadRequest("Error parsing before_ts: " + err.Error())
		}
	}

	params := mux.Vars(r)

	datastoreId := params["datastoreId"]

	log = log.WithFields(logrus.Fields{
		"beforeTs":    beforeTs,
		"datastoreId": datastoreId,
	})

	result, err := maintenance_controller.EstimateDatastoreSizeWithAge(beforeTs, datastoreId, r.Context(), log)
	if err != nil {
		log.Error(err)
		return api.InternalServerError("Unexpected error getting storage estimate")
	}
	return &api.DoNotCacheResponse{Payload: result}
}
