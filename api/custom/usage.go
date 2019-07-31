package custom

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type MinimalUsageInfo struct {
	Total int64 `json:"total"`
	Media int64 `json:"media"`
}

type UsageInfo struct {
	*MinimalUsageInfo
	Thumbnails int64 `json:"thumbnails"`
}

type CountsUsageResponse struct {
	RawBytes  *UsageInfo `json:"raw_bytes"`
	RawCounts *UsageInfo `json:"raw_counts"`
}

type UserUsageEntry struct {
	RawBytes     *MinimalUsageInfo `json:"raw_bytes"`
	RawCounts    *MinimalUsageInfo `json:"raw_counts"`
	UploadedMxcs []string          `json:"uploaded,flow"`
}

type MediaUsageEntry struct {
	SizeBytes         int64  `json:"size_bytes"`
	UploadedBy        string `json:"uploaded_by"`
	DatastoreId       string `json:"datastore_id"`
	DatastoreLocation string `json:"datastore_location"`
	Sha256Hash        string `json:"sha256_hash"`
	Quarantined       bool   `json:"quarantined"`
	UploadName        string `json:"upload_name"`
	ContentType       string `json:"content_type"`
	CreatedTs         int64  `json:"created_ts"`
}

func GetDomainUsage(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	params := mux.Vars(r)

	serverName := params["serverName"]

	log = log.WithFields(logrus.Fields{
		"serverName": serverName,
	})

	db := storage.GetDatabase().GetMetadataStore(r.Context(), log)

	mediaBytes, thumbBytes, err := db.GetByteUsageForServer(serverName)
	if err != nil {
		log.Error(err)
		return api.InternalServerError("Failed to get byte usage for server")
	}

	mediaCount, thumbCount, err := db.GetCountUsageForServer(serverName)
	if err != nil {
		log.Error(err)
		return api.InternalServerError("Failed to get count usage for server")
	}

	return &CountsUsageResponse{
		RawBytes: &UsageInfo{
			MinimalUsageInfo: &MinimalUsageInfo{
				Total: mediaBytes + thumbBytes,
				Media: mediaBytes,
			},
			Thumbnails: thumbBytes,
		},
		RawCounts: &UsageInfo{
			MinimalUsageInfo: &MinimalUsageInfo{
				Total: mediaCount + thumbCount,
				Media: mediaCount,
			},
			Thumbnails: thumbCount,
		},
	}
}

func GetUserUsage(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	params := mux.Vars(r)

	serverName := params["serverName"]
	userIds := r.URL.Query()["user_id"]

	log = log.WithFields(logrus.Fields{
		"serverName": serverName,
	})

	db := storage.GetDatabase().GetMediaStore(r.Context(), log)

	var records []*types.Media
	var err error
	if userIds == nil || len(userIds) == 0 {
		records, err = db.GetAllMediaForServer(serverName)
	} else {
		records, err = db.GetAllMediaForServerUsers(serverName, userIds)
	}

	if err != nil {
		log.Error(err)
		return api.InternalServerError("Failed to get media records for users")
	}

	parsed := make(map[string]*UserUsageEntry)

	for _, media := range records {
		var entry *UserUsageEntry
		var ok bool
		if entry, ok = parsed[media.UserId]; !ok {
			entry = &UserUsageEntry{
				UploadedMxcs: make([]string, 0),
				RawCounts: &MinimalUsageInfo{
					Total: 0,
					Media: 0,
				},
				RawBytes: &MinimalUsageInfo{
					Total: 0,
					Media: 0,
				},
			}
			parsed[media.UserId] = entry
		}

		entry.RawBytes.Total += media.SizeBytes
		entry.RawBytes.Media += media.SizeBytes
		entry.RawCounts.Total += 1
		entry.RawCounts.Media += 1

		entry.UploadedMxcs = append(entry.UploadedMxcs, media.MxcUri())
	}

	return parsed
}

func GetUploadsUsage(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	params := mux.Vars(r)

	serverName := params["serverName"]
	mxcs := r.URL.Query()["mxc"]

	log = log.WithFields(logrus.Fields{
		"serverName": serverName,
	})

	db := storage.GetDatabase().GetMediaStore(r.Context(), log)

	var records []*types.Media
	var err error
	if mxcs == nil || len(mxcs) == 0 {
		records, err = db.GetAllMediaForServer(serverName)
	} else {
		split := make([]string, 0)
		for _, mxc := range mxcs {
			o, i, err := util.SplitMxc(mxc)
			if err != nil {
				log.Error(err)
				return api.InternalServerError("Error parsing MXC " + mxc)
			}

			if o != serverName {
				return api.BadRequest("MXC URIs must match the requested server")
			}

			split = append(split, i)
		}
		records, err = db.GetAllMediaInIds(serverName, split)
	}

	if err != nil {
		log.Error(err)
		return api.InternalServerError("Failed to get media records for users")
	}

	parsed := make(map[string]*MediaUsageEntry)

	for _, media := range records {
		parsed[media.MxcUri()] = &MediaUsageEntry{
			SizeBytes:         media.SizeBytes,
			UploadName:        media.UploadName,
			ContentType:       media.ContentType,
			CreatedTs:         media.CreationTs,
			DatastoreId:       media.DatastoreId,
			DatastoreLocation: media.Location,
			Quarantined:       media.Quarantined,
			Sha256Hash:        media.Sha256Hash,
			UploadedBy:        media.UserId,
		}
	}

	return parsed
}
