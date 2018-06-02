package media_cache

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/services/media_service"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

func (c *mediaCache) getKeyForMedia(server string, mediaId string) string {
	return fmt.Sprintf("media:%s_%s", server, mediaId)
}

func (c *mediaCache) getMediaRecordId(media *types.Media) string {
	return fmt.Sprintf("media:%s_%s", media.Origin, media.MediaId)
}

func (c *mediaCache) GetMedia(server string, mediaId string, downloadRemote bool) (*types.StreamedMedia, error) {
	media, err := c.GetRawMedia(server, mediaId, downloadRemote)
	if err != nil {
		return nil, err
	}

	if media.Quarantined {
		c.log.Warn("Quarantined media accessed")
		return nil, common.ErrMediaQuarantined
	}

	// At this point we should have a real media object to use, so let's try caching it
	c.incrementMediaDownloads(media)
	cachedFile, err := c.updateMediaInCache(media)
	if err != nil {
		return nil, err
	}

	if cachedFile != nil {
		return &types.StreamedMedia{Media: media, Stream: util.BufferToStream(cachedFile.contents)}, nil
	}

	c.log.Info("Using media from disk")
	stream, err := os.Open(media.Location)
	if err != nil {
		return nil, err
	}

	return &types.StreamedMedia{Media: media, Stream: stream}, nil
}

func (c *mediaCache) GetRawMedia(server string, mediaId string, downloadRemote bool) (*types.Media, error) {
	mediaSvc := media_service.New(c.ctx, c.log)

	// First see if we have the media in cache
	if config.Get().Downloads.Cache.Enabled {
		item, found := c.cache.Get(c.getKeyForMedia(server, mediaId))
		if found {
			m := item.(*cachedFile)
			if m.media == nil {
				return nil, errors.New("expected a cached media object but got a cached thumbnail")
			}

			c.log.Info("Using cached media")
			return m.media, nil
		}
	}

	// We proxy the call for media, so we'll at least try and get it first
	c.log.Info("Searching for media")
	media, err := mediaSvc.GetMediaDirect(server, mediaId)
	if err != nil {
		if err == sql.ErrNoRows {
			if util.IsServerOurs(server) {
				c.log.Warn("Media not found")
				return nil, common.ErrMediaNotFound
			}
		}

		if !downloadRemote {
			c.log.Warn("Remote media not being downloaded")
			return nil, common.ErrMediaNotFound
		}

		media, err = mediaSvc.GetRemoteMediaDirect(server, mediaId)
		if err != nil {
			return nil, err
		}
	}

	exists, err := util.FileExists(media.Location)
	if !exists || err != nil {
		if util.IsServerOurs(server) {
			c.log.Error("Media not found in file store when we expected it to")
			return nil, common.ErrMediaNotFound
		} else {
			if !downloadRemote {
				c.log.Warn("Media appears to have been deleted, however we're not redownloading it")
				return nil, common.ErrMediaNotFound
			}

			c.log.Warn("Media appears to have been deleted - redownloading")

			media, err = mediaSvc.GetRemoteMediaDirect(server, mediaId)
			if err != nil {
				return nil, err
			}
		}
	}

	return media, nil
}

func (c *mediaCache) incrementMediaDownloads(media *types.Media) {
	if !config.Get().Downloads.Cache.Enabled {
		return // Not enabled - don't bother
	}

	c.tracker.Increment(c.getMediaRecordId(media))
}

func (c *mediaCache) updateMediaInCache(media *types.Media) (*cachedFile, error) {
	if !config.Get().Downloads.Cache.Enabled {
		return nil, nil // Not enabled - don't bother (not cached)
	}

	log := c.log.WithFields(logrus.Fields{
		"cache_origin":    media.Origin,
		"cache_mediaId":   media.MediaId,
		"cache_mediaSize": media.SizeBytes,
	})

	cacheFn := func() (*cachedFile, error) { return c.cacheMedia(media) }
	cacheKey := c.getKeyForMedia(media.Origin, media.MediaId)
	recordId := c.getMediaRecordId(media)
	return c.updateItemInCache(cacheKey, recordId, media.SizeBytes, cacheFn, log)
}

func (c *mediaCache) cacheMedia(media *types.Media) (*cachedFile, error) {
	data, err := ioutil.ReadFile(media.Location)
	if err != nil {
		return nil, err
	}

	record := &cachedFile{media: media, contents: bytes.NewBuffer(data)}
	c.cache.Set(c.getKeyForMedia(media.Origin, media.MediaId), record, cache.DefaultExpiration)
	return record, nil
}
