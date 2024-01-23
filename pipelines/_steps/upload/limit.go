package upload

import (
	"io"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/util/readers"
)

func LimitStream(ctx rcontext.RequestContext, r io.ReadCloser) io.ReadCloser {
	if ctx.Config.Uploads.MaxSizeBytes > 0 {
		return readers.LimitReaderWithOverrunError(r, ctx.Config.Uploads.MaxSizeBytes)
	} else {
		return r
	}
}
