package upload_pipeline

import (
	"io"
	"io/ioutil"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

func limitStreamLength(ctx rcontext.RequestContext, r io.ReadCloser) io.ReadCloser {
	if ctx.Config.Uploads.MaxSizeBytes > 0 {
		return ioutil.NopCloser(io.LimitReader(r, ctx.Config.Uploads.MaxSizeBytes))
	} else {
		return r
	}
}
