package task_runner

import (
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
)

func PurgePreviews(ctx rcontext.RequestContext) {
	// dev note: don't use ctx for config lookup to avoid misreading it

	if config.Get().UrlPreviews.ExpireDays <= 0 {
		return
	}

	before := time.Now().AddDate(0, 0, -1*config.Get().UrlPreviews.ExpireDays)
	db := database.GetInstance().UrlPreviews.Prepare(ctx)

	// TODO: Fix https://github.com/t2bot/matrix-media-repo/issues/424 ("can't clean up preview media")
	if err := db.DeleteOlderThan(before); err != nil {
		ctx.Log.Error("Error deleting previews: ", err)
		sentry.CaptureException(err)
	}
}
