package meta

import (
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/metrics"
)

func FlagAccess(ctx rcontext.RequestContext, sha256hash string, uploadTime int64) {
	uploaded := time.UnixMilli(uploadTime)
	if uploadTime > 0 {
		metrics.MediaAgeAccessed.Observe(time.Since(uploaded).Seconds())
	}
	if err := database.GetInstance().LastAccess.Prepare(ctx).Upsert(sha256hash, time.Now().UnixMilli()); err != nil {
		ctx.Log.Warnf("Non-fatal error while updating last access for '%s': %v", sha256hash, err)
		sentry.CaptureException(err)
	}
}
