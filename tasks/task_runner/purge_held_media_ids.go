package task_runner

import (
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
)

func PurgeHeldMediaIds(ctx rcontext.RequestContext) {
	// dev note: don't use ctx for config lookup to avoid misreading it

	before := time.Now().AddDate(0, 0, -7)
	db := database.GetInstance().HeldMedia.Prepare(ctx)

	if err := db.DeleteOlderThan(database.ForCreateHeldReason, before); err != nil {
		ctx.Log.Error("Error deleting held media IDs: ", err)
		sentry.CaptureException(err)
	}
}
