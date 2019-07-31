package custom

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/storage"
)

type UsageInfo struct {
	Total      int64 `json:"total"`
	Media      int64 `json:"media"`
	Thumbnails int64 `json:"thumbnails"`
}

type DomainUsageResponse struct {
	RawBytes  UsageInfo `json:"raw_bytes"`
	RawCounts UsageInfo `json:"raw_counts"`
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

	return &DomainUsageResponse{
		RawBytes: UsageInfo{
			Total:      mediaBytes + thumbBytes,
			Media:      mediaBytes,
			Thumbnails: thumbBytes,
		},
		RawCounts: UsageInfo{
			Total:      mediaCount + thumbCount,
			Media:      mediaCount,
			Thumbnails: thumbCount,
		},
	}
}
