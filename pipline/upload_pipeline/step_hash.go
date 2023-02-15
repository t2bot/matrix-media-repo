package upload_pipeline

import (
	"io"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/util/stream_util"
)

func hashFile(ctx rcontext.RequestContext, r io.ReadCloser) (string, error) {
	return stream_util.GetSha256HashOfStream(r)
}
