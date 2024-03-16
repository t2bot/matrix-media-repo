package upload

import (
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
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
