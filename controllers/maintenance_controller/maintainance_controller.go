package maintenance_controller

import (
	"database/sql"
	"os"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/download_controller"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

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
	media, err := download_controller.FindMediaRecord(origin, mediaId, false, ctx)
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
