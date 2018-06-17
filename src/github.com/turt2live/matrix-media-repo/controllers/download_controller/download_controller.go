package download_controller

import (
	"context"
	"database/sql"
	"os"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/internal_cache"
	"github.com/turt2live/matrix-media-repo/matrix"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

var localCache = cache.New(30*time.Second, 60*time.Second)

func GetMedia(origin string, mediaId string, downloadRemote bool, bearerToken *types.BearerToken, ctx context.Context, log *logrus.Entry) (*types.StreamedMedia, error) {
	media, err := FindMediaRecord(origin, mediaId, downloadRemote, ctx, log)
	if err != nil {
		return nil, err
	}

	if media.ContentToken != nil {
		log.Info("Media is protected by a content token - verifying request")
		userId, err := matrix.GetUserIdFromBearerToken(ctx, bearerToken, *media.ContentToken)
		if err != nil {
			return nil, err
		}

		log.Info("Access token belongs to ", userId)
	}

	if media.Quarantined {
		log.Warn("Quarantined media accessed")
		return nil, common.ErrMediaQuarantined
	}

	localCache.Set(origin+"/"+mediaId, media, cache.DefaultExpiration)
	internal_cache.Get().IncrementDownloads(media.Sha256Hash)

	cached, err := internal_cache.Get().GetMedia(media, log)
	if err != nil {
		return nil, err
	}
	if cached != nil && cached.Contents != nil {
		return &types.StreamedMedia{
			Media:  media,
			Stream: util.BufferToStream(cached.Contents),
		}, nil
	}

	log.Info("Reading media from disk")
	stream, err := os.Open(media.Location)
	if err != nil {
		return nil, err
	}

	return &types.StreamedMedia{Media: media, Stream: stream}, nil
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

			result := <-getResourceHandler().DownloadRemoteMedia(origin, mediaId)
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
