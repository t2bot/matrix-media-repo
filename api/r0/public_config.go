package r0

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/api/apimeta"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/pipelines/steps/quota"
)

type PublicConfigResponse struct {
	UploadMaxSize   int64 `json:"m.upload.size,omitempty"`
	StorageMaxSize  int64 `json:"org.matrix.msc4034.storage.size,omitempty"`
	StorageMaxFiles int64 `json:"org.matrix.msc4034.storage.max_files,omitempty"`
}

func PublicConfig(r *http.Request, rctx rcontext.RequestContext, user apimeta.UserInfo) interface{} {
	uploadSize := rctx.Config.Uploads.ReportedMaxSizeBytes
	if uploadSize == 0 {
		uploadSize = rctx.Config.Uploads.MaxSizeBytes
	}

	if uploadSize < 0 {
		uploadSize = 0 // invokes the omitEmpty
	}

	storageSize := int64(0)
	limit, err := quota.Limit(rctx, user.UserId, quota.MaxBytes)
	if err != nil {
		rctx.Log.Warn("Non-fatal error getting per-user quota limit (max bytes): ", err)
		sentry.CaptureException(err)
	} else {
		storageSize = limit
	}
	if storageSize < 0 {
		storageSize = 0 // invokes the omitEmpty
	}

	maxFiles := int64(0)
	limit, err = quota.Limit(rctx, user.UserId, quota.MaxCount)
	if err != nil {
		rctx.Log.Warn("Non-fatal error getting per-user quota limit (max files count): ", err)
		sentry.CaptureException(err)
	} else {
		maxFiles = limit
	}

	return &PublicConfigResponse{
		UploadMaxSize:   uploadSize,
		StorageMaxSize:  storageSize,
		StorageMaxFiles: maxFiles,
	}
}
