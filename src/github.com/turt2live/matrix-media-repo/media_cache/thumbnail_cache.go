package media_cache

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/services/thumbnail_service"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

func (c *mediaCache) getKeyForThumbnail(server string, mediaId string, width int, height int, method string, animated bool) string {
	return fmt.Sprintf("thumbnail:%s_%s_%d_%d_%s_%t", server, mediaId, width, height, method, animated)
}

func (c *mediaCache) getThumbnailRecordId(thumbnail *types.Thumbnail) string {
	return fmt.Sprintf("thumbnail:%s_%s_%d_%d_%s_%t", thumbnail.Origin, thumbnail.MediaId, thumbnail.Width, thumbnail.Height, thumbnail.Method, thumbnail.Animated)
}

func (c *mediaCache) GetThumbnail(server string, mediaId string, width int, height int, method string, animated bool) (*types.StreamedThumbnail, error) {
	width, height, method, err := c.pickThumbnailDimensions(width, height, method)
	if err != nil {
		return nil, err
	}

	thumbnail, err := c.GetRawThumbnail(server, mediaId, width, height, method, animated)
	if err != nil {
		return nil, err
	}

	// At this point we should have a real thumbnail to use, so let's try caching it
	c.incrementThumbnailDownloads(thumbnail)
	cachedMedia, err := c.updateThumbnailInCache(thumbnail)
	if err != nil {
		return nil, err
	}

	if cachedMedia != nil {
		return &types.StreamedThumbnail{Thumbnail: thumbnail, Stream: util.GetStreamFromBuffer(cachedMedia.contents)}, nil
	}

	c.log.Info("Using thumbnail from disk")
	stream, err := os.Open(thumbnail.Location)
	if err != nil {
		return nil, err
	}

	return &types.StreamedThumbnail{Thumbnail: thumbnail, Stream: stream}, nil
}

func (c *mediaCache) GetRawThumbnail(server string, mediaId string, width int, height int, method string, animated bool) (*types.Thumbnail, error) {
	width, height, method, err := c.pickThumbnailDimensions(width, height, method)
	if err != nil {
		return nil, err
	}

	thumbnailSvc := thumbnail_service.New(c.ctx, c.log)

	// First see if the thumbnail is already in the cache
	if config.Get().Downloads.Cache.Enabled {
		item, found := c.cache.Get(c.getKeyForThumbnail(server, mediaId, width, height, method, animated))
		if found {
			m := item.(*cachedFile)
			if m.thumbnail == nil {
				return nil, errors.New("expected a cached thumbnail but got cached media")
			}

			c.log.Info("Using cached thumbnail")
			return m.thumbnail, nil
		}
	}

	// We proxy the call for thumbnails, so we'll at least try and get it
	media, err := c.GetRawMedia(server, mediaId)
	if err != nil {
		return nil, err
	}

	thumb, err := thumbnailSvc.GetThumbnailDirect(media, width, height, method, animated)
	if err != nil && err != sql.ErrNoRows {
		c.log.Error("Unexpected error getting thumbnail: " + err.Error())
		return nil, err
	}
	if err != sql.ErrNoRows {
		c.log.Info("Using existing thumbnail")
		return thumb, nil
	}

	// At this point we'll try and generate the thumbnail
	thumb, err = thumbnailSvc.GenerateThumbnail(media, width, height, method, animated)
	if err != nil {
		return nil, err
	}

	return thumb, nil
}

func (c *mediaCache) pickThumbnailDimensions(desiredWidth int, desiredHeight int, desiredMethod string) (int, int, string, error) {
	if desiredWidth <= 0 {
		return 0, 0, "", errors.New("width must be positive")
	}
	if desiredHeight <= 0 {
		return 0, 0, "", errors.New("height must be positive")
	}
	if desiredMethod != "crop" && desiredMethod != "scale" {
		return 0, 0, "", errors.New("method must be crop or scale")
	}

	targetWidth := desiredWidth
	targetHeight := desiredHeight
	foundFirst := false

	for i := 0; i < len(config.Get().Thumbnails.Sizes); i++ {
		size := config.Get().Thumbnails.Sizes[i]
		lastSize := i == len(config.Get().Thumbnails.Sizes)-1

		if desiredWidth == size.Width && desiredHeight == size.Height {
			targetWidth = desiredWidth
			targetHeight = desiredHeight
			break
		}

		if (size.Width < desiredWidth || size.Height < desiredHeight) && !lastSize {
			continue // too small
		}

		diffWidth := size.Width - desiredWidth
		diffHeight := size.Height - desiredHeight
		currDiffWidth := targetWidth - desiredWidth
		currDiffHeight := targetHeight - desiredHeight

		diff := diffWidth + diffHeight
		currDiff := currDiffWidth + currDiffHeight

		if !foundFirst || diff < currDiff || lastSize {
			foundFirst = true
			targetWidth = size.Width
			targetHeight = size.Height
		}
	}

	return targetWidth, targetHeight, desiredMethod, nil
}

func (c *mediaCache) incrementThumbnailDownloads(thumbnail *types.Thumbnail) {
	if !config.Get().Downloads.Cache.Enabled {
		return // Not enabled - don't bother
	}

	c.tracker.Increment(c.getThumbnailRecordId(thumbnail))
}

func (c *mediaCache) updateThumbnailInCache(thumbnail *types.Thumbnail) (*cachedFile, error) {
	if !config.Get().Downloads.Cache.Enabled {
		return nil, nil // Not enabled - don't bother (not cached)
	}

	log := c.log.WithFields(logrus.Fields{
		"cache_origin":        thumbnail.Origin,
		"cache_mediaId":       thumbnail.MediaId,
		"cache_thumbnailSize": thumbnail.SizeBytes,
		"cache_width":         thumbnail.Width,
		"cache_height":        thumbnail.Height,
		"cache_method":        thumbnail.Method,
		"cache_animated":      thumbnail.Animated,
	})

	cacheFn := func() (*cachedFile, error) { return c.cacheThumbnail(thumbnail) }
	cacheKey := c.getKeyForThumbnail(thumbnail.Origin, thumbnail.MediaId, thumbnail.Width, thumbnail.Height, thumbnail.Method, thumbnail.Animated)
	recordId := c.getThumbnailRecordId(thumbnail)
	return c.updateItemInCache(cacheKey, recordId, thumbnail.SizeBytes, cacheFn, log)
}

func (c *mediaCache) cacheThumbnail(thumbnail *types.Thumbnail) (*cachedFile, error) {
	data, err := ioutil.ReadFile(thumbnail.Location)
	if err != nil {
		return nil, err
	}

	record := &cachedFile{thumbnail: thumbnail, contents: bytes.NewBuffer(data)}
	cacheKey := c.getKeyForThumbnail(thumbnail.Origin, thumbnail.MediaId, thumbnail.Width, thumbnail.Height, thumbnail.Method, thumbnail.Animated)
	c.cache.Set(cacheKey, record, cache.DefaultExpiration)
	return record, nil
}
