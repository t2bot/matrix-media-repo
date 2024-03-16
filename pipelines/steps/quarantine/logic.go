package quarantine

import (
	"io"

	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

func ReturnAppropriateThing(ctx rcontext.RequestContext, isDownload bool, recordOnly bool, width int, height int) (io.ReadCloser, error) {
	flag := ctx.Config.Quarantine.ReplaceDownloads
	if !isDownload {
		flag = ctx.Config.Quarantine.ReplaceThumbnails
	}
	if !flag || recordOnly {
		return nil, common.ErrMediaQuarantined
	} else {
		return MakeThumbnail(ctx, width, height)
	}
}
