package quarantine

import (
	"io"

	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/pipelines/_steps/download"
)

func ReturnAppropriateThing(ctx rcontext.RequestContext, isDownload bool, recordOnly bool, width int, height int, startByte int64, endByte int64) (io.ReadCloser, error) {
	flag := ctx.Config.Quarantine.ReplaceDownloads
	if !isDownload {
		flag = ctx.Config.Quarantine.ReplaceThumbnails
	}
	if !flag || recordOnly {
		return nil, common.ErrMediaQuarantined
	} else {
		if qr, err := MakeThumbnail(ctx, width, height); err != nil {
			return nil, err
		} else {
			if r, err2 := download.CreateLimitedStream(ctx, qr, startByte, endByte); err2 != nil {
				return nil, err2
			} else {
				return r, common.ErrMediaQuarantined
			}
		}
	}
}
