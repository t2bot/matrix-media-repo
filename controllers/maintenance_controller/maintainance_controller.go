package maintenance_controller

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

func StartStorageMigration(sourceDs *datastore.DatastoreRef, targetDs *datastore.DatastoreRef, beforeTs int64, log *logrus.Entry) {
	ctx := context.Background()
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

		log.Info("Finished transfer")
	}()
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
	}

	return removed, nil
}
