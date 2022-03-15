package maintenance_controller

import (
	"database/sql"
	"fmt"
	"github.com/getsentry/sentry-go"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/download_controller"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

// Returns an error only if starting up the background task failed.
func StartStorageMigration(sourceDs *datastore.DatastoreRef, targetDs *datastore.DatastoreRef, beforeTs int64, ctx rcontext.RequestContext) (*types.BackgroundTask, error) {
	db := storage.GetDatabase().GetMetadataStore(ctx)
	task, err := db.CreateBackgroundTask("storage_migration", map[string]interface{}{
		"source_datastore_id": sourceDs.DatastoreId,
		"target_datastore_id": targetDs.DatastoreId,
		"before_ts":           beforeTs,
	})
	if err != nil {
		return nil, err
	}

	go func() {
		ctx.Log.Info("Starting transfer")

		db := storage.GetDatabase().GetMetadataStore(ctx)

		doUpdate := func(records []*types.MinimalMediaMetadata) {
			for _, record := range records {
				rctx := ctx.LogWithFields(logrus.Fields{"mediaSha256": record.Sha256Hash})

				rctx.Log.Info("Starting transfer of media")
				sourceStream, err := sourceDs.DownloadFile(record.Location)
				if err != nil {
					rctx.Log.Error(err)
					rctx.Log.Error("Failed to start download from source datastore")
					sentry.CaptureException(err)
					continue
				}

				newLocation, err := targetDs.UploadFile(sourceStream, record.SizeBytes, rctx)
				if err != nil {
					rctx.Log.Error(err)
					rctx.Log.Error("Failed to upload file to target datastore")
					sentry.CaptureException(err)
					continue
				}

				if newLocation.Sha256Hash != record.Sha256Hash {
					rctx.Log.Error("sha256 hash does not match - not moving media")
					sentry.CaptureMessage("sha256 hash does not match - not moving media")
					targetDs.DeleteObject(newLocation.Location)
					continue
				}

				rctx.Log.Info("Updating media records...")
				err = db.ChangeDatastoreOfHash(targetDs.DatastoreId, newLocation.Location, record.Sha256Hash)
				if err != nil {
					rctx.Log.Error(err)
					rctx.Log.Error("Failed to update database records")
					sentry.CaptureException(err)
					continue
				}

				rctx.Log.Info("Deleting media from old datastore")
				err = sourceDs.DeleteObject(record.Location)
				if err != nil {
					rctx.Log.Error(err)
					rctx.Log.Error("Failed to delete old media")
					sentry.CaptureException(err)
					continue
				}

				rctx.Log.Info("Media updated!")
			}
		}

		media, err := db.GetOldMediaInDatastore(sourceDs.DatastoreId, beforeTs)
		if err != nil {
			ctx.Log.Error(err)
			sentry.CaptureException(err)
			return
		}
		doUpdate(media)

		thumbs, err := db.GetOldThumbnailsInDatastore(sourceDs.DatastoreId, beforeTs)
		if err != nil {
			ctx.Log.Error(err)
			sentry.CaptureException(err)
			return
		}
		doUpdate(thumbs)

		err = db.FinishedBackgroundTask(task.ID)
		if err != nil {
			ctx.Log.Error(err)
			ctx.Log.Error("Failed to flag task as finished")
			sentry.CaptureException(err)
		}
		ctx.Log.Info("Finished transfer")
	}()

	return task, nil
}

func EstimateDatastoreSizeWithAge(beforeTs int64, datastoreId string, ctx rcontext.RequestContext) (*types.DatastoreMigrationEstimate, error) {
	estimates := &types.DatastoreMigrationEstimate{}
	seenHashes := make(map[string]bool)
	seenMediaHashes := make(map[string]bool)
	seenThumbnailHashes := make(map[string]bool)

	db := storage.GetDatabase().GetMetadataStore(ctx)
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

func PurgeRemoteMediaBefore(beforeTs int64, ctx rcontext.RequestContext) (int, error) {
	db := storage.GetDatabase().GetMediaStore(ctx)
	thumbsDb := storage.GetDatabase().GetThumbnailStore(ctx)

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

	ctx.Log.Info(fmt.Sprintf("Starting removal of %d remote media files (db records will be kept)", len(oldMedia)))

	removed := 0
	for _, media := range oldMedia {
		if media.Quarantined {
			ctx.Log.Warn("Not removing quarantined media to maintain quarantined status: " + media.Origin + "/" + media.MediaId)
			continue
		}

		ds, err := datastore.LocateDatastore(ctx, media.DatastoreId)
		if err != nil {
			ctx.Log.Error("Error finding datastore for media " + media.Origin + "/" + media.MediaId + " because: " + err.Error())
			sentry.CaptureException(err)
			continue
		}

		// Delete the file first
		err = ds.DeleteObject(media.Location)
		if err != nil {
			ctx.Log.Warn("Cannot remove media " + media.Origin + "/" + media.MediaId + " because: " + err.Error())
			sentry.CaptureException(err)
		} else {
			removed++
			ctx.Log.Info("Removed remote media file: " + media.Origin + "/" + media.MediaId)
		}

		// Try to remove the record from the database now
		err = db.Delete(media.Origin, media.MediaId)
		if err != nil {
			ctx.Log.Warn("Error removing media " + media.Origin + "/" + media.MediaId + " from database: " + err.Error())
			sentry.CaptureException(err)
		}

		// Delete the thumbnails too
		thumbs, err := thumbsDb.GetAllForMedia(media.Origin, media.MediaId)
		if err != nil {
			ctx.Log.Warn("Error getting thumbnails for media " + media.Origin + "/" + media.MediaId + " from database: " + err.Error())
			sentry.CaptureException(err)
			continue
		}
		for _, thumb := range thumbs {
			ctx.Log.Info("Deleting thumbnail with hash: ", thumb.Sha256Hash)
			ds, err := datastore.LocateDatastore(ctx, thumb.DatastoreId)
			if err != nil {
				ctx.Log.Warn("Error removing thumbnail for media " + media.Origin + "/" + media.MediaId + " from database: " + err.Error())
				sentry.CaptureException(err)
				continue
			}

			err = ds.DeleteObject(thumb.Location)
			if err != nil {
				ctx.Log.Warn("Error removing thumbnail for media " + media.Origin + "/" + media.MediaId + " from database: " + err.Error())
				sentry.CaptureException(err)
				continue
			}
		}
		err = thumbsDb.DeleteAllForMedia(media.Origin, media.MediaId)
		if err != nil {
			ctx.Log.Warn("Error removing thumbnails for media " + media.Origin + "/" + media.MediaId + " from database: " + err.Error())
			sentry.CaptureException(err)
		}
	}

	return removed, nil
}

func PurgeQuarantined(ctx rcontext.RequestContext) ([]*types.Media, error) {
	mediaDb := storage.GetDatabase().GetMediaStore(ctx)
	records, err := mediaDb.GetAllQuarantinedMedia()
	if err != nil {
		return nil, err
	}

	for _, r := range records {
		err = doPurge(r, ctx)
		if err != nil {
			return nil, err
		}
	}

	return records, nil
}

func PurgeQuarantinedFor(serverName string, ctx rcontext.RequestContext) ([]*types.Media, error) {
	mediaDb := storage.GetDatabase().GetMediaStore(ctx)
	records, err := mediaDb.GetQuarantinedMediaFor(serverName)
	if err != nil {
		return nil, err
	}

	for _, r := range records {
		err = doPurge(r, ctx)
		if err != nil {
			return nil, err
		}
	}

	return records, nil
}

func PurgeUserMedia(userId string, beforeTs int64, ctx rcontext.RequestContext) ([]*types.Media, error) {
	mediaDb := storage.GetDatabase().GetMediaStore(ctx)
	records, err := mediaDb.GetMediaByUserBefore(userId, beforeTs)
	if err != nil {
		return nil, err
	}

	for _, r := range records {
		err = doPurge(r, ctx)
		if err != nil {
			return nil, err
		}
	}

	return records, nil
}

func PurgeOldMedia(beforeTs int64, includeLocal bool, ctx rcontext.RequestContext) ([]*types.Media, error) {
	metadataDb := storage.GetDatabase().GetMetadataStore(ctx)
	mediaDb := storage.GetDatabase().GetMediaStore(ctx)

	oldHashes, err := metadataDb.GetOldMedia(beforeTs)
	if err != nil {
		return nil, err
	}

	purged := make([]*types.Media, 0)

	for _, r := range oldHashes {
		media, err := mediaDb.GetByHash(r.Sha256Hash)
		if err != nil {
			return nil, err
		}

		for _, m := range media {
			if !includeLocal && util.IsServerOurs(m.Origin) {
				continue
			}

			err = doPurge(m, ctx)
			if err != nil {
				return nil, err
			}

			purged = append(purged, m)
		}
	}

	return purged, nil
}

func PurgeRoomMedia(mxcs []string, beforeTs int64, ctx rcontext.RequestContext) ([]*types.Media, error) {
	mediaDb := storage.GetDatabase().GetMediaStore(ctx)

	purged := make([]*types.Media, 0)

	// we have to manually find each record because the SQL query is too complex
	for _, mxc := range mxcs {
		domain, mediaId, err := util.SplitMxc(mxc)
		if err != nil {
			return nil, err
		}

		record, err := mediaDb.Get(domain, mediaId)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}

		if record.CreationTs > beforeTs {
			continue
		}

		err = doPurge(record, ctx)
		if err != nil {
			return nil, err
		}

		purged = append(purged, record)
	}

	return purged, nil
}

func PurgeDomainMedia(serverName string, beforeTs int64, ctx rcontext.RequestContext) ([]*types.Media, error) {
	mediaDb := storage.GetDatabase().GetMediaStore(ctx)
	records, err := mediaDb.GetMediaByDomainBefore(serverName, beforeTs)
	if err != nil {
		return nil, err
	}

	for _, r := range records {
		err = doPurge(r, ctx)
		if err != nil {
			return nil, err
		}
	}

	return records, nil
}

func PurgeMedia(origin string, mediaId string, ctx rcontext.RequestContext) error {
	media, err := download_controller.FindMediaRecord(origin, mediaId, false, nil, ctx)
	if err != nil {
		return err
	}

	return doPurge(media, ctx)
}

func doPurge(media *types.Media, ctx rcontext.RequestContext) error {
	// Delete all the thumbnails first
	thumbsDb := storage.GetDatabase().GetThumbnailStore(ctx)
	thumbs, err := thumbsDb.GetAllForMedia(media.Origin, media.MediaId)
	if err != nil {
		return err
	}
	for _, thumb := range thumbs {
		ctx.Log.Info("Deleting thumbnail with hash: ", thumb.Sha256Hash)
		ds, err := datastore.LocateDatastore(ctx, thumb.DatastoreId)
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

	ds, err := datastore.LocateDatastore(ctx, media.DatastoreId)
	if err != nil {
		return err
	}

	mediaDb := storage.GetDatabase().GetMediaStore(ctx)
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
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	} else {
		ctx.Log.Warnf("Not deleting media from datastore: media is shared over %d objects", len(similarMedia))
	}

	metadataDb := storage.GetDatabase().GetMetadataStore(ctx)

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
