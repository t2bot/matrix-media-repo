package download_controller

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/internal_cache"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

var localCache = cache.New(30*time.Second, 60*time.Second)

func GetMedia(origin string, mediaId string, downloadRemote bool, blockForMedia bool, ctx context.Context, log *logrus.Entry) (*types.MinimalMedia, error) {
	var media *types.Media
	var minMedia *types.MinimalMedia
	var err error
	if blockForMedia {
		media, err = FindMediaRecord(origin, mediaId, downloadRemote, ctx, log)
		if media != nil {
			minMedia = &types.MinimalMedia{
				Origin:      media.Origin,
				MediaId:     media.MediaId,
				ContentType: media.ContentType,
				UploadName:  media.UploadName,
				SizeBytes:   media.SizeBytes,
				Stream:      nil, // we'll populate this later if we need to
				KnownMedia:  media,
			}
		}
	} else {
		minMedia, err = FindMinimalMediaRecord(origin, mediaId, downloadRemote, ctx, log)
		if minMedia != nil {
			media = minMedia.KnownMedia
		}
	}
	if err != nil {
		return nil, err
	}
	if minMedia == nil {
		log.Warn("Unexpected error while fetching media: no minimal media record")
		return nil, common.ErrMediaNotFound
	}
	if media == nil && blockForMedia {
		log.Warn("Unexpected error while fetching media: no regular media record (block for media in place)")
		return nil, common.ErrMediaNotFound
	}

	// if we have a known media record, we might as well set it
	// if we don't, this won't do anything different
	minMedia.KnownMedia = media

	if media != nil {
		if media.Quarantined {
			log.Warn("Quarantined media accessed")
			return nil, common.ErrMediaQuarantined
		}

		err = storage.GetDatabase().GetMetadataStore(ctx, log).UpsertLastAccess(media.Sha256Hash, util.NowMillis())
		if err != nil {
			logrus.Warn("Failed to upsert the last access time: ", err)
		}

		localCache.Set(origin+"/"+mediaId, media, cache.DefaultExpiration)
		internal_cache.Get().IncrementDownloads(media.Sha256Hash)

		cached, err := internal_cache.Get().GetMedia(media, log)
		if err != nil {
			return nil, err
		}
		if cached != nil && cached.Contents != nil {
			minMedia.Stream = util.BufferToStream(cached.Contents)
			return minMedia, nil
		}
	}

	if minMedia.Stream != nil {
		log.Info("Returning minimal media record with a viable stream")
		return minMedia, nil
	}

	if media == nil {
		log.Error("Failed to locate media")
		return nil, errors.New("failed to locate media")
	}

	log.Info("Reading media from disk")
	mediaStream, err := datastore.DownloadStream(ctx, log, media.DatastoreId, media.Location)
	if err != nil {
		return nil, err
	}

	minMedia.Stream = mediaStream
	return minMedia, nil
}

func FindMinimalMediaRecord(origin string, mediaId string, downloadRemote bool, ctx context.Context, log *logrus.Entry) (*types.MinimalMedia, error) {
	db := storage.GetDatabase().GetMediaStore(ctx, log)

	var media *types.Media
	item, found := localCache.Get(origin + "/" + mediaId)
	if found {
		media = item.(*types.Media)
	} else {
		log.Info("Getting media record from database")
		dbMedia, err := db.Get(origin, mediaId)
		if err != nil {
			if err == sql.ErrNoRows {
				if util.IsServerOurs(origin) {
					log.Warn("Media not found")
					return nil, common.ErrMediaNotFound
				}
			}

			if !downloadRemote {
				log.Warn("Remote media not being downloaded")
				return nil, common.ErrMediaNotFound
			}

			result := <-getResourceHandler().DownloadRemoteMedia(origin, mediaId, false)
			if result.err != nil {
				return nil, result.err
			}
			return &types.MinimalMedia{
				Origin:      origin,
				MediaId:     mediaId,
				ContentType: result.contentType,
				UploadName:  result.filename,
				SizeBytes:   -1, // unknown
				Stream:      result.stream,
				KnownMedia:  nil, // unknown
			}, nil
		} else {
			media = dbMedia
		}
	}

	if media == nil {
		log.Warn("Despite all efforts, a media record could not be found")
		return nil, common.ErrMediaNotFound
	}

	mediaStream, err := datastore.DownloadStream(ctx, log, media.DatastoreId, media.Location)
	if err != nil {
		return nil, err
	}

	return &types.MinimalMedia{
		Origin:      media.Origin,
		MediaId:     media.MediaId,
		ContentType: media.ContentType,
		UploadName:  media.UploadName,
		SizeBytes:   media.SizeBytes,
		Stream:      mediaStream,
		KnownMedia:  media,
	}, nil
}

func FindMediaRecord(origin string, mediaId string, downloadRemote bool, ctx context.Context, log *logrus.Entry) (*types.Media, error) {
	db := storage.GetDatabase().GetMediaStore(ctx, log)

	var media *types.Media
	item, found := localCache.Get(origin + "/" + mediaId)
	if found {
		media = item.(*types.Media)
	} else {
		log.Info("Getting media record from database")
		dbMedia, err := db.Get(origin, mediaId)
		if err != nil {
			if err == sql.ErrNoRows {
				if util.IsServerOurs(origin) {
					log.Warn("Media not found")
					return nil, common.ErrMediaNotFound
				}
			}

			if !downloadRemote {
				log.Warn("Remote media not being downloaded")
				return nil, common.ErrMediaNotFound
			}

			result := <-getResourceHandler().DownloadRemoteMedia(origin, mediaId, true)
			if result.err != nil {
				return nil, result.err
			}
			media = result.media
		} else {
			media = dbMedia
		}
	}

	if media == nil {
		log.Warn("Despite all efforts, a media record could not be found")
		return nil, common.ErrMediaNotFound
	}

	return media, nil
}
