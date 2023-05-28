package upload

import (
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
)

func CheckQuarantineStatus(ctx rcontext.RequestContext, hash string) error {
	q, err := database.GetInstance().Media.Prepare(ctx).IsHashQuarantined(hash)
	if err != nil {
		return err
	}
	if q {
		return common.ErrMediaQuarantined
	}
	return nil
}
