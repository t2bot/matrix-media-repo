package custom

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/util"
)

type DatastoreMigrationEstimate struct {
	ThumbnailsAffected      int64 `json:"thumbnails_affected"`
	ThumbnailHashesAffected int64 `json:"thumbnail_hashes_affected"`
	ThumbnailBytes          int64 `json:"thumbnail_bytes"`
	MediaAffected           int64 `json:"media_affected"`
	MediaHashesAffected     int64 `json:"media_hashes_affected"`
	MediaBytes              int64 `json:"media_bytes"`
	TotalHashesAffected     int64 `json:"total_hashes_affected"`
	TotalBytes              int64 `json:"total_bytes"`
}

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

	return response
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

	estimates := &DatastoreMigrationEstimate{}
	seenHashes := make(map[string]bool)
	seenMediaHashes := make(map[string]bool)
	seenThumbnailHashes := make(map[string]bool)

	db := storage.GetDatabase().GetMetadataStore(r.Context(), log)
	media, err := db.GetOldMediaInDatastore(datastoreId, beforeTs)
	if err != nil {
		log.Error(err)
		return api.InternalServerError("Failed to get media from database")
	}

	for _, record := range media {
		estimates.MediaAffected++

		if _, found := seenHashes[record.Sha256Hash]; !found {
			estimates.TotalBytes += record.SizeBytes
			estimates.TotalHashesAffected++
		}
		if _, found := seenMediaHashes[record.Sha256Hash]; !found {
			estimates.MediaBytes += record.SizeBytes
			estimates.MediaHashesAffected++
		}

		seenHashes[record.Sha256Hash] = true
		seenMediaHashes[record.Sha256Hash] = true
	}

	thumbnails, err := db.GetOldMediaInDatastore(datastoreId, beforeTs)
	for _, record := range thumbnails {
		estimates.ThumbnailsAffected++

		if _, found := seenHashes[record.Sha256Hash]; !found {
			estimates.TotalBytes += record.SizeBytes
			estimates.TotalHashesAffected++
		}
		if _, found := seenThumbnailHashes[record.Sha256Hash]; !found {
			estimates.ThumbnailBytes += record.SizeBytes
			estimates.ThumbnailHashesAffected++
		}

		seenHashes[record.Sha256Hash] = true
		seenThumbnailHashes[record.Sha256Hash] = true
	}

	return estimates
}
