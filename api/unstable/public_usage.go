package unstable

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/pipelines/_steps/quota"
)

type PublicUsageResponse struct {
	StorageFree  int64 `json:"org.matrix.msc4034.storage.free,omitempty"`
	StorageFiles int64 `json:"org.matrix.msc4034.storage.files,omitempty"`
}

func PublicUsage(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	storageUsed := int64(0)
	storageLimit := int64(0)
	limit, err := quota.Limit(rctx, user.UserId, quota.MaxBytes)
	if err != nil {
		rctx.Log.Warn("Non-fatal error getting per-user quota limit (max bytes): ", err)
		sentry.CaptureException(err)
	} else if limit > 0 {
		storageLimit = limit
	}
	if storageLimit > 0 {
		current, err := quota.Current(rctx, user.UserId, quota.MaxBytes)
		if err != nil {
			rctx.Log.Warn("Non-fatal error getting per-user quota usage (max bytes @ now): ", err)
			sentry.CaptureException(err)
		} else {
			storageUsed = current
		}
	} else {
		storageLimit = 0
	}

	fileCount, err := quota.Current(rctx, user.UserId, quota.MaxCount)
	if err != nil {
		rctx.Log.Warn("Non-fatal error getting per-user quota usage (files count @ now): ", err)
		sentry.CaptureException(err)
	}

	return &PublicUsageResponse{
		StorageFree:  storageLimit - storageUsed,
		StorageFiles: fileCount,
	}
}
