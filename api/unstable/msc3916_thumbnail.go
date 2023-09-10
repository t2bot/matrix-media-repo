package unstable

import (
	"bytes"
	"net/http"

	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/api/r0"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/util/readers"
)

func FederationThumbnailMedia(r *http.Request, rctx rcontext.RequestContext, server _apimeta.ServerInfo) interface{} {
	r.URL.Query().Set("allow_remote", "false")

	res := r0.ThumbnailMedia(r, rctx, _apimeta.UserInfo{})
	if dl, ok := res.(*_responses.DownloadResponse); ok {
		return &_responses.DownloadResponse{
			ContentType: "multipart/mixed",
			Filename:    "",
			SizeBytes:   0,
			Data: readers.NewMultipartReader(
				&readers.MultipartPart{ContentType: "application/json", Reader: readers.MakeCloser(bytes.NewReader([]byte("{}")))},
				&readers.MultipartPart{ContentType: dl.ContentType, FileName: dl.Filename, Reader: dl.Data},
			),
			TargetDisposition: "attachment",
		}
	} else {
		return res
	}
}
