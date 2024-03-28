package custom

import (
	"encoding/json"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/database"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

type UserQuotaEntry struct {
	MaxBytes   int64 `json:"max_bytes"`
	MaxPending int64 `json:"max_pending"`
	MaxFiles   int64 `json:"max_files"`
}

func GetUserQuota(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	userIds := r.URL.Query()["user_id"]

	db := database.GetInstance().UserStats.Prepare(rctx)

	records, err := db.GetUserQuota(userIds)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Failed to get quota for users")
	}

	parsed := make(map[string]*UserQuotaEntry)

	for _, quota := range records {
		entry := &UserQuotaEntry{
			MaxBytes:   quota.UserQuota.MaxBytes,
			MaxPending: quota.UserQuota.MaxPending,
			MaxFiles:   quota.UserQuota.MaxFiles,
		}
		parsed[quota.UserId] = entry
	}

	return &_responses.DoNotCacheResponse{Payload: parsed}
}

func SetUserQuota(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	decoder := json.NewDecoder(r.Body)
	params := make(map[string]*UserQuotaEntry)
	err := decoder.Decode(&params)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Failed to read SetUserQuota parameters")
	}

	db := database.GetInstance().UserStats.Prepare(rctx)

	for userId, quota := range params {
		if quota.MaxBytes < -1 || quota.MaxFiles < -1 || quota.MaxPending < -1 {
			rctx.Log.Warn("SetUserQuota parameters for user " + userId + " must be >= -1. Skipping...")
			continue
		}

		err = db.SetUserQuota(userId, quota.MaxBytes, quota.MaxFiles, quota.MaxPending)
		if err != nil {
			rctx.Log.Error(err)
			sentry.CaptureException(err)
			return _responses.InternalServerError("Failed to set quota for user " + userId)
		}
	}

	return &_responses.DoNotCacheResponse{Payload: &_responses.EmptyResponse{}}
}
