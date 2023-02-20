package r0

import (
	"net/http"

	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/upload_controller"
)

type PublicConfigResponse struct {
	UploadMaxSize int64 `json:"m.upload.size,omitempty"`
}

func PublicConfig(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	uploadSize := rctx.Config.Uploads.ReportedMaxSizeBytes
	if uploadSize == 0 {
		if !rctx.Config.Uploads.MaxBytesPerUser.Enabled {
			uploadSize = rctx.Config.Uploads.MaxSizeBytes
		} else {
			uploadSize = upload_controller.GetUploadMaxBytesForUser(rctx, user.UserId)
		}
	}

	if uploadSize < 0 {
		uploadSize = 0 // invokes the omitEmpty
	}

	return &PublicConfigResponse{
		UploadMaxSize: uploadSize,
	}
}
