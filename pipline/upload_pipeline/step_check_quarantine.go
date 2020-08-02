package upload_pipeline

import (
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/storage"
)

func checkQuarantineStatus(ctx rcontext.RequestContext, hash string) error {
	db := storage.GetDatabase().GetMediaStore(ctx)
	q, err := db.IsQuarantined(hash)
	if err != nil {
		return err
	}
	if q {
		return common.ErrMediaQuarantined
	}
	return nil
}
