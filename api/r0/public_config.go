package r0

import (
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common/config"
)

type PublicConfigResponse struct {
	UploadMaxSize int64 `json:"m.upload.size,omitempty"`
}

func PublicConfig(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	uploadSize := config.Get().Uploads.ReportedMaxSizeBytes
	if uploadSize == 0 {
		uploadSize = config.Get().Uploads.MaxSizeBytes
	}

	if uploadSize < 0 {
		uploadSize = 0 // invokes the omitEmpty
	}

	return &PublicConfigResponse{
		UploadMaxSize: uploadSize,
	}
}
