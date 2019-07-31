package custom

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/types"
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

		entry.UploadedMxcs = append(entry.UploadedMxcs, fmt.Sprintf("mxc://%s/%s", media.Origin, media.MediaId))
	}

	return parsed
}
