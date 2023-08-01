package task_runner

import (
	"errors"
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/datastores"
	"github.com/turt2live/matrix-media-repo/util"
)

func PurgeThumbnails(ctx rcontext.RequestContext) {
	// dev note: don't use ctx for config lookup to avoid misreading it

	if config.Get().Thumbnails.ExpireDays <= 0 {
		return
	}

	beforeTs := util.NowMillis() - int64(config.Get().UrlPreviews.ExpireDays*24*60*60*1000)
	db := database.GetInstance().Thumbnails.Prepare(ctx)
	old, err := db.GetOlderThan(beforeTs)
	if err != nil {
		ctx.Log.Error("Error deleting thumbnails: ", err)
		sentry.CaptureException(err)
		return
	}

	mediaDb := database.GetInstance().Media.Prepare(ctx)
	deletedLocations := make(map[string]bool)
	for _, thumb := range old {
		mxc := fmt.Sprintf("%s?w=%d&h=%d&m=%s&a=%t", util.MxcUri(thumb.Origin, thumb.MediaId), thumb.Width, thumb.Height, thumb.Method, thumb.Animated)
		ctx.Log.Debugf("Trying to purge thumbnail %s", mxc)
		if exists, err := mediaDb.LocationExists(thumb.DatastoreId, thumb.Location); err != nil {
			ctx.Log.Error("Error checking for conflicting media: ", err)
			sentry.CaptureException(err)
		} else if !exists { // if exists, skip
			locationId := fmt.Sprintf("%s/%s", thumb.DatastoreId, thumb.Location)
			if _, ok := deletedLocations[locationId]; !ok {
				ds, ok := datastores.Get(ctx, thumb.DatastoreId)
				if !ok {
					ctx.Log.Errorf("Unable to locate datastore '%s' for thumbnail '%s'", thumb.DatastoreId, mxc)
					sentry.CaptureException(errors.New("unable to locate datastore for thumbnail"))
					continue
				}
				ctx.Log.Debugf("Trying to remove datastore object for %s", mxc)
				err = datastores.Remove(ctx, ds, thumb.Location)
				if err != nil {
					ctx.Log.Error("Error deleting thumbnail from datastore: ", err)
					sentry.CaptureException(err)
					continue
				}
				deletedLocations[locationId] = true
			}
			ctx.Log.Debugf("Trying to database record for %s", mxc)
			if err = db.Delete(thumb); err != nil {
				ctx.Log.Error("Error deleting thumbnail record: ", err)
				sentry.CaptureException(err)
			}
		}
	}
}
