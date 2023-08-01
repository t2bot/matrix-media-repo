package task_runner

import (
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/datastores"
	"github.com/turt2live/matrix-media-repo/util"
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
	thumbsDb := database.GetInstance().Thumbnails.Prepare(ctx)

	origins := util.GetOurDomains()

	records, err := mediaDb.GetOldExcluding(origins, beforeTs)
	if err != nil {
		return 0, err
	}

	removed := 0
	deletedLocations := make(map[string]bool)
	for _, record := range records {
		mxc := util.MxcUri(record.Origin, record.MediaId)
		if record.Quarantined {
			ctx.Log.Debugf("Skipping quarantined media %s", mxc)
			continue // skip quarantined media
		}

		if exists, err := thumbsDb.LocationExists(record.DatastoreId, record.Location); err != nil {
			ctx.Log.Error("Error checking for conflicting thumbnail: ", err)
			sentry.CaptureException(err)
		} else if !exists { // if exists, skip
			locationId := fmt.Sprintf("%s/%s", record.DatastoreId, record.Location)
			if _, ok := deletedLocations[locationId]; !ok {
				ctx.Log.Debugf("Trying to remove datastore object for %s", mxc)
				err = datastores.RemoveWithDsId(ctx, record.DatastoreId, record.Location)
				if err != nil {
					ctx.Log.Error("Error deleting media from datastore: ", err)
					sentry.CaptureException(err)
					continue
				}
				deletedLocations[locationId] = true
			}
			ctx.Log.Debugf("Trying to database record for %s", mxc)
			if err = mediaDb.Delete(record.Origin, record.MediaId); err != nil {
				ctx.Log.Error("Error deleting thumbnail record: ", err)
				sentry.CaptureException(err)
			}
			removed = removed + 1

			thumbs, err := thumbsDb.GetForMedia(record.Origin, record.MediaId)
			if err != nil {
				ctx.Log.Warn("Error getting thumbnails for media: ", err)
				sentry.CaptureException(err)
				continue
			}

			doPurgeThumbnails(ctx, thumbs)
		}
	}

	return removed, nil
}
