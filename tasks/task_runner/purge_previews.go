package task_runner

import (
	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/util"
)

func PurgePreviews(ctx rcontext.RequestContext) {
	// dev note: don't use ctx for config lookup to avoid misreading it

	if config.Get().UrlPreviews.ExpireDays <= 0 {
		return
	}

	beforeTs := util.NowMillis() - int64(config.Get().UrlPreviews.ExpireDays*24*60*60*1000)
	db := database.GetInstance().UrlPreviews.Prepare(ctx)

	// TODO: Fix https://github.com/turt2live/matrix-media-repo/issues/424 ("can't clean up preview media")
	if err := db.DeleteOlderThan(beforeTs); err != nil {
		ctx.Log.Error("Error deleting previews: ", err)
		sentry.CaptureException(err)
	}
}
