package upload_pipeline

import (
	"io"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

func bufferStream(ctx rcontext.RequestContext, r io.ReadCloser) ([]byte, error) {
	return io.ReadAll(r)
}
