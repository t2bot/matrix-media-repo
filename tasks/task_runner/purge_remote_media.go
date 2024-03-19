package task_runner

import (
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

	beforeTs := util.NowMillis() - int64(config.Get().Downloads.ExpireDays*24*60*60*1000)
	_, err := PurgeRemoteMediaBefore(ctx, beforeTs)
	if err != nil {
		ctx.Log.Error("Error purging media: ", err)
		sentry.CaptureException(err)
	}
}

// PurgeRemoteMediaBefore returns (count affected, error)
func PurgeRemoteMediaBefore(ctx rcontext.RequestContext, beforeTs int64) (int, error) {
	mediaDb := database.GetInstance().Media.Prepare(ctx)

	origins := util.GetOurDomains()

	records, err := mediaDb.GetOldExcluding(origins, beforeTs)
	if err != nil {
		return 0, err
	}

	removed, err := doPurge(ctx.AsBackground(), records, &purgeConfig{IncludeQuarantined: false})
	if err != nil {
		return 0, err
	}

	return len(removed), nil
}
