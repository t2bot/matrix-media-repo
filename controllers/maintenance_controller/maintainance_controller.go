package maintenance_controller

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/controllers/download_controller"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

// Returns an error only if starting up the background task failed.
func StartStorageMigration(sourceDs *datastore.DatastoreRef, targetDs *datastore.DatastoreRef, beforeTs int64, log *logrus.Entry) (*types.BackgroundTask, error) {
	ctx := context.Background()

	db := storage.GetDatabase().GetMetadataStore(ctx, log)
	task, err := db.CreateBackgroundTask("storage_migration", map[string]interface{}{
		"source_datastore_id": sourceDs.DatastoreId,
		"target_datastore_id": targetDs.DatastoreId,
		"before_ts":           beforeTs,
	})
	if err != nil {
		return nil, err
	}

	go func() {
		log.Info("Starting transfer")

		db := storage.GetDatabase().GetMetadataStore(ctx, log)

		origLog := log
		doUpdate := func(records []*types.MinimalMediaMetadata) {
			for _, record := range records {
				log := origLog.WithFields(logrus.Fields{"mediaSha256": record.Sha256Hash})

				log.Info("Starting transfer of media")
				sourceStream, err := sourceDs.DownloadFile(record.Location)
				if err != nil {
					log.Error(err)
					log.Error("Failed to start download from source datastore")
					continue
				}

				newLocation, err := targetDs.UploadFile(sourceStream, record.SizeBytes, ctx, log)
				if err != nil {
					log.Error(err)
					log.Error("Failed to upload file to target datastore")
					continue
				}

				log.Info("Updating media records...")
				err = db.ChangeDatastoreOfHash(targetDs.DatastoreId, newLocation.Location, record.Sha256Hash)
				if err != nil {
					log.Error(err)
					log.Error("Failed to update database records")
					continue
				}

				log.Info("Deleting media from old datastore")
				err = sourceDs.DeleteObject(record.Location)
				if err != nil {
					log.Error(err)
					log.Error("Failed to delete old media")
					continue
				}

				log.Info("Media updated!")
			}
		}

		media, err := db.GetOldMediaInDatastore(sourceDs.DatastoreId, beforeTs)
		if err != nil {
			log.Error(err)
			return
		}
		doUpdate(media)

		thumbs, err := db.GetOldThumbnailsInDatastore(sourceDs.DatastoreId, beforeTs)
		if err != nil {
			log.Error(err)
			return
		}
		doUpdate(thumbs)

		err = db.FinishedBackgroundTask(task.ID)
		if err != nil {
			log.Error(err)
			log.Error("Failed to flag task as finished")
		}
		log.Info("Finished transfer")
	}()

	return task, nil
}

func EstimateDatastoreSizeWithAge(beforeTs int64, datastoreId string, ctx context.Context, log *logrus.Entry) (*types.DatastoreMigrationEstimate, error) {
	estimates := &types.DatastoreMigrationEstimate{}
	seenHashes := make(map[string]bool)
	seenMediaHashes := make(map[string]bool)
	seenThumbnailHashes := make(map[string]bool)

	db := storage.GetDatabase().GetMetadataStore(ctx, log)
	media, err := db.GetOldMediaInDatastore(datastoreId, beforeTs)
	if err != nil {
		return nil, err
	}

	for _, record := range media {
		estimates.MediaAffected++

		if _, found := seenHashes[record.Sha256Hash]; !found {
			estimates.TotalBytes += record.SizeBytes
			estimates.TotalHashesAffected++
		}
		if _, found := seenMediaHashes[record.Sha256Hash]; !found {
			estimates.MediaBytes += record.SizeBytes
			estimates.MediaHashesAffected++
		}

		seenHashes[record.Sha256Hash] = true
		seenMediaHashes[record.Sha256Hash] = true
	}

	thumbnails, err := db.GetOldThumbnailsInDatastore(datastoreId, beforeTs)
	if err != nil {
		return nil, err
	}

	for _, record := range thumbnails {
		estimates.ThumbnailsAffected++

		if _, found := seenHashes[record.Sha256Hash]; !found {
			estimates.TotalBytes += record.SizeBytes
			estimates.TotalHashesAffected++
		}
		if _, found := seenThumbnailHashes[record.Sha256Hash]; !found {
			estimates.ThumbnailBytes += record.SizeBytes
			estimates.ThumbnailHashesAffected++
		}

		seenHashes[record.Sha256Hash] = true
		seenThumbnailHashes[record.Sha256Hash] = true
	}

	return estimates, nil
}

func PurgeRemoteMediaBefore(beforeTs int64, ctx context.Context, log *logrus.Entry) (int, error) {
	db := storage.GetDatabase().GetMediaStore(ctx, log)
	thumbsDb := storage.GetDatabase().GetThumbnailStore(ctx, log)

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

		ds, err := datastore.LocateDatastore(context.TODO(), &logrus.Entry{}, media.DatastoreId)
		if err != nil {
			log.Error("Error finding datastore for media " + media.Origin + "/" + media.MediaId + " because: " + err.Error())
			continue
		}

		// Delete the file first
		err = ds.DeleteObject(media.Location)
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

		// Delete the thumbnails too
		thumbs, err := thumbsDb.GetAllForMedia(media.Origin, media.MediaId)
		if err != nil {
			log.Warn("Error getting thumbnails for media " + media.Origin + "/" + media.MediaId + " from database: " + err.Error())
			continue
		}
		for _, thumb := range thumbs {
			log.Info("Deleting thumbnail with hash: ", thumb.Sha256Hash)
			ds, err := datastore.LocateDatastore(ctx, log, thumb.DatastoreId)
			if err != nil {
				log.Warn("Error removing thumbnail for media " + media.Origin + "/" + media.MediaId + " from database: " + err.Error())
				continue
			}

			err = ds.DeleteObject(thumb.Location)
			if err != nil {
				log.Warn("Error removing thumbnail for media " + media.Origin + "/" + media.MediaId + " from database: " + err.Error())
				continue
			}
		}
		err = thumbsDb.DeleteAllForMedia(media.Origin, media.MediaId)
		if err != nil {
			log.Warn("Error removing thumbnails for media " + media.Origin + "/" + media.MediaId + " from database: " + err.Error())
		}
	}

	return removed, nil
}

func PurgeQuarantined(ctx context.Context, log *logrus.Entry) ([]*types.Media, error) {
	mediaDb := storage.GetDatabase().GetMediaStore(ctx, log)
	records, err := mediaDb.GetAllQuarantinedMedia()
	if err != nil {
		return nil, err
	}

	for _, r := range records {
		err = doPurge(r, ctx, log)
		if err != nil {
			return nil, err
		}
	}

	return records, nil
}

func PurgeQuarantinedFor(serverName string, ctx context.Context, log *logrus.Entry) ([]*types.Media, error) {
	mediaDb := storage.GetDatabase().GetMediaStore(ctx, log)
	records, err := mediaDb.GetQuarantinedMediaFor(serverName)
	if err != nil {
		return nil, err
	}

	for _, r := range records {
		err = doPurge(r, ctx, log)
		if err != nil {
			return nil, err
		}
	}

	return records, nil
}

func PurgeMedia(origin string, mediaId string, ctx context.Context, log *logrus.Entry) error {
	media, err := download_controller.FindMediaRecord(origin, mediaId, false, ctx, log)
	if err != nil {
		return err
	}

	return doPurge(media, ctx, log)
}

func doPurge(media *types.Media, ctx context.Context, log *logrus.Entry) error {
	// Delete all the thumbnails first
	thumbsDb := storage.GetDatabase().GetThumbnailStore(ctx, log)
	thumbs, err := thumbsDb.GetAllForMedia(media.Origin, media.MediaId)
	if err != nil {
		return err
	}
	for _, thumb := range thumbs {
		log.Info("Deleting thumbnail with hash: ", thumb.Sha256Hash)
		ds, err := datastore.LocateDatastore(ctx, log, thumb.DatastoreId)
		if err != nil {
			return err
		}

		err = ds.DeleteObject(thumb.Location)
		if err != nil {
			return err
		}
	}
	err = thumbsDb.DeleteAllForMedia(media.Origin, media.MediaId)
	if err != nil {
		return err
	}

	ds, err := datastore.LocateDatastore(ctx, log, media.DatastoreId)
	if err != nil {
		return err
	}

	mediaDb := storage.GetDatabase().GetMediaStore(ctx, log)
	similarMedia, err := mediaDb.GetByHash(media.Sha256Hash)
	if err != nil {
		return err
	}
	hasSimilar := false
	for _, m := range similarMedia {
		if m.Origin != media.Origin && m.MediaId != media.MediaId {
			hasSimilar = true
			break
		}
	}

	if !hasSimilar || media.Quarantined {
		err = ds.DeleteObject(media.Location)
		if err != nil {
			return err
		}
	} else {
		log.Warnf("Not deleting media from datastore: media is shared over %d objects", len(similarMedia))
	}

	metadataDb := storage.GetDatabase().GetMetadataStore(ctx, log)

	reserved, err := metadataDb.IsReserved(media.Origin, media.MediaId)
	if err != nil {
		return err
	}

	if !reserved {
		err = metadataDb.ReserveMediaId(media.Origin, media.MediaId, "purged / deleted")
		if err != nil {
			return err
		}
	}

	// Don't delete the media record itself if it is quarantined. If we delete it, the media
	// becomes not-quarantined so we'll leave it and let it 404 in the datastores.
	if media.Quarantined {
		return nil
	}

	err = mediaDb.Delete(media.Origin, media.MediaId)
	if err != nil {
		return err
	}

	return nil
}
