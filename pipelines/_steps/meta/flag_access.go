package meta

import (
	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/util"
)

func FlagAccess(ctx rcontext.RequestContext, sha256hash string) {
	if err := database.GetInstance().LastAccess.Prepare(ctx).Upsert(sha256hash, util.NowMillis()); err != nil {
		ctx.Log.Warnf("Non-fatal error while updating last access for '%s': %s", sha256hash, err.Error())
		sentry.CaptureException(err)
	}
}
