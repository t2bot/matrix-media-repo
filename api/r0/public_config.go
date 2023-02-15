package r0

import (
	"net/http"

	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

type PublicConfigResponse struct {
	UploadMaxSize int64 `json:"m.upload.size,omitempty"`
}

func PublicConfig(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	uploadSize := rctx.Config.Uploads.ReportedMaxSizeBytes
	if uploadSize == 0 {
		uploadSize = rctx.Config.Uploads.MaxSizeBytes
	}

	if uploadSize < 0 {
		uploadSize = 0 // invokes the omitEmpty
	}

	return &PublicConfigResponse{
		UploadMaxSize: uploadSize,
	}
}
