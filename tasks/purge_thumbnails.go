package tasks

import (
	"math/rand"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/util"
)

var thumbnailsPurgeDone chan bool

func StartThumbnailPurgeRecurring() {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	ticker := time.NewTicker((1 * time.Hour) + (time.Duration(r.Intn(15)) * time.Minute))
	thumbnailsPurgeDone = make(chan bool)

	go func() {
		for {
			select {
			case <-thumbnailsPurgeDone:
				ticker.Stop()
				return
			case <-ticker.C:
				if config.Get().Thumbnails.ExpireDays <= 0 {
					continue
				}

				doRecurringThumbnailPurge()
			}
		}
	}()
}

func StopThumbnailPurgeRecurring() {
	thumbnailsPurgeDone <- true
}

func doRecurringThumbnailPurge() {
	ctx := rcontext.Initial().LogWithFields(logrus.Fields{"task": "recurring_purge_thumbnails"})
	ctx.Log.Info("Starting thumbnail purge task")

	// We get media that is N days old to make sure it gets cleared safely.
	beforeTs := util.NowMillis() - int64(config.Get().Thumbnails.ExpireDays*24*60*60*1000)

	db := storage.GetDatabase().GetThumbnailStore(ctx)
	thumbs, err := db.GetOldThumbnails(beforeTs)
	if err != nil {
		ctx.Log.Error(err)
		return
	}

	mediaDb := storage.GetDatabase().GetMediaStore(ctx)

	for _, thumb := range thumbs {
		// Double check that the thumbnail won't also delete some media
		m, err := mediaDb.GetMediaByLocation(thumb.DatastoreId, thumb.Location)
		if err != nil {
			ctx.Log.Error(err)
			return
		}
		if len(m) > 0 {
			ctx.Log.Warnf("Refusing to delete thumbnails with hash %s because it looks like other pieces of media are using it", thumb.Sha256Hash)
			continue
		}

		ctx.Log.Info("Deleting thumbnails with hash: ", thumb.Sha256Hash)
		err = db.DeleteWithHash(thumb.Sha256Hash)
		if err != nil {
			ctx.Log.Error(err)
			return
		}

		ds, err := datastore.LocateDatastore(ctx, thumb.DatastoreId)
		if err != nil {
			ctx.Log.Error(err)
			return
		}

		err = ds.DeleteObject(thumb.Location)
		if err != nil {
			ctx.Log.Error(err)
			// don't return on this one - we'll continue otherwise
		}
	}

	ctx.Log.Info("Purge task completed")
}
