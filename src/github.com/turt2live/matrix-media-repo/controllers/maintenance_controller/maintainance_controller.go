package maintenance_controller

import (
	"context"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/util"
)

func PurgeRemoteMediaBefore(beforeTs int64, ctx context.Context, log *logrus.Entry) (int, error) {
	db := storage.GetDatabase().GetMediaStore(ctx, log)

	origins, err := db.GetOrigins()
	if err != nil {
		return 0, err
	}

	var excludedOrigins []string
	for _, origin := range origins {
		if util.IsServerOurs(origin) {
			excludedOrigins = append(excludedOrigins, origin)
		}
	}

	oldMedia, err := db.GetOldMedia(excludedOrigins, beforeTs)
	if err != nil {
		return 0, err
	}

	log.Info(fmt.Sprintf("Starting removal of %d remote media files (db records will be kept)", len(oldMedia)))

	removed := 0
	for _, media := range oldMedia {
		if media.Quarantined {
			log.Warn("Not removing quarantined media to maintain quarantined status: " + media.Origin + "/" + media.MediaId)
			continue
		}

		// Delete the file first
		err = os.Remove(media.Location)
		if err != nil {
			log.Warn("Cannot remove media " + media.Origin + "/" + media.MediaId + " because: " + err.Error())
		} else {
			removed++
			log.Info("Removed remote media file: " + media.Origin + "/" + media.MediaId)
		}

		// Try to remove the record from the database now
		err = db.Delete(media.Origin, media.MediaId)
		if err != nil {
			log.Warn("Error removing media " + media.Origin + "/" + media.MediaId + " from database: " + err.Error())
		}
	}

	return removed, nil
}
