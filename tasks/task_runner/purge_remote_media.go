package task_runner

import (
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/util"
)

func PurgeRemoteMedia(ctx rcontext.RequestContext) {
	// dev note: don't use ctx for config lookup to avoid misreading it

	if config.Get().Downloads.ExpireDays <= 0 {
		return
	}

	before := time.Now().AddDate(0, 0, -1*config.Get().Downloads.ExpireDays)
	_, err := PurgeRemoteMediaBefore(ctx, before)
	if err != nil {
		ctx.Log.Error("Error purging media: ", err)
		sentry.CaptureException(err)
	}
}

// PurgeRemoteMediaBefore returns (count affected, error)
func PurgeRemoteMediaBefore(ctx rcontext.RequestContext, beforeTs time.Time) (int, error) {
	mediaDb := database.GetInstance().Media.Prepare(ctx)

	origins := util.GetOurDomains()

	records, err := mediaDb.GetOldExcluding(origins, beforeTs)
	if err != nil {
		return 0, err
	}

	removed, err := doPurge(ctx, records, &purgeConfig{IncludeQuarantined: false})
	if err != nil {
		return 0, err
	}

	return len(removed), nil
}
