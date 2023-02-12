package r0

import (
	"net/http"

	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/quota"
)

type PublicConfigResponse struct {
	UploadMaxSize int64 `json:"m.upload.size,omitempty"`
}

func PublicConfig(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	uploadSize := quota.GetUserUploadMaxSizeBytes(rctx, user.UserId)
	return &PublicConfigResponse{
		UploadMaxSize: uploadSize,
	}
}
