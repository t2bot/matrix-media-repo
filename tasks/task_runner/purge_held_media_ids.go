package task_runner

import (
	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/util"
)

func PurgeHeldMediaIds(ctx rcontext.RequestContext) {
	// dev note: don't use ctx for config lookup to avoid misreading it

	beforeTs := util.NowMillis() - int64(7*24*60*60*1000) // 7 days
	db := database.GetInstance().HeldMedia.Prepare(ctx)

	if err := db.DeleteOlderThan(database.ForCreateHeldReason, beforeTs); err != nil {
		ctx.Log.Error("Error deleting held media IDs: ", err)
		sentry.CaptureException(err)
	}
}
