package media_cache

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"

	"github.com/disintegration/imaging"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/old_middle_layer/services/thumbnail_service"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

var regexContentType = regexp.MustCompile(`^\s*([^\/]+)\/(?:[^\.]+\.)?([^+;\s]+).*$`)

func (c *mediaCache) getKeyForThumbnail(server string, mediaId string, width int, height int, method string, animated bool) string {
	return fmt.Sprintf("thumbnail:%s_%s_%d_%d_%s_%t", server, mediaId, width, height, method, animated)
}

func (c *mediaCache) getThumbnailRecordId(thumbnail *types.Thumbnail) string {
	return fmt.Sprintf("thumbnail:%s_%s_%d_%d_%s_%t", thumbnail.Origin, thumbnail.MediaId, thumbnail.Width, thumbnail.Height, thumbnail.Method, thumbnail.Animated)
}

func (c *mediaCache) GetThumbnail(server string, mediaId string, width int, height int, method string, animated bool, downloadRemote bool) (*types.StreamedThumbnail, error) {
	width, height, method, err := c.pickThumbnailDimensions(width, height, method)
	if err != nil {
		return nil, err
	}

	thumbnail, err := c.GetRawThumbnail(server, mediaId, width, height, method, animated, downloadRemote)
	if err != nil {
		if err == common.ErrMediaQuarantined {
			c.log.Warn("Quarantined media accessed")
		}

		if err == common.ErrMediaQuarantined && config.Get().Quarantine.ReplaceThumbnails {
			c.log.Info("Replacing thumbnail with a quarantined icon")
			svc := thumbnail_service.New(c.ctx, c.log)
			img, err := svc.GenerateQuarantineThumbnail(server, mediaId, width, height)
			if err != nil {
				return nil, err
			}

			data := &bytes.Buffer{}
			imaging.Encode(data, img, imaging.PNG)
			return &types.StreamedThumbnail{
				Stream: util.BufferToStream(data),
				Thumbnail: &types.Thumbnail{
					// We lie about most of these details to maintain the contract
					Width:       img.Bounds().Max.X,
					Height:      img.Bounds().Max.Y,
					MediaId:     mediaId,
					Origin:      server,
					Location:    "",
					ContentType: "image/png",
					Animated:    false,
					Method:      method,
					CreationTs:  util.NowMillis(),
					SizeBytes:   int64(data.Len()),
				},
			}, nil
		}

		return nil, err
	}

	// fake the thumbnail.Animated with what was requested so that cache lookups actually work
	thumbAnimated := thumbnail.Animated
	thumbnail.Animated = animated

	// At this point we should have a real thumbnail to use, so let's try caching it
	c.incrementThumbnailDownloads(thumbnail)
	cachedMedia, err := c.updateThumbnailInCache(thumbnail)
	if err != nil {
		return nil, err
	}

	// restore our actual thumbnail.Animation
	thumbnail.Animated = thumbAnimated

	if cachedMedia != nil {
		return &types.StreamedThumbnail{Thumbnail: thumbnail, Stream: util.BufferToStream(cachedMedia.contents)}, nil
	}

	c.log.Info("Using thumbnail from disk")
	stream, err := os.Open(thumbnail.Location)
	if err != nil {
		return nil, err
	}

	return &types.StreamedThumbnail{Thumbnail: thumbnail, Stream: stream}, nil
}

func (c *mediaCache) GetRawThumbnail(server string, mediaId string, width int, height int, method string, animated bool, downloadRemote bool) (*types.Thumbnail, error) {
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
	media, err := c.GetRawMedia(server, mediaId, downloadRemote)
	if err != nil {
		return nil, err
	}

	// now we need to sanitize the mimetype (ContentType)
	media.ContentType = regexContentType.ReplaceAllString(media.ContentType, `$1/$2`)

	if animated && !util.ArrayContains(thumbnail_service.AnimatedTypes, media.ContentType) {
		c.log.Warn("Cannot animate a non-animated file. Assuming animated=false")
		animated = false
	}

	if media.Quarantined {
		return nil, common.ErrMediaQuarantined
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

	foundSize := false
	targetWidth := 0
	targetHeight := 0
	largestWidth := 0
	largestHeight := 0

	for _, size := range config.Get().Thumbnails.Sizes {
		largestWidth = util.MaxInt(largestWidth, size.Width)
		largestHeight = util.MaxInt(largestHeight, size.Height)

		// Unlikely, but if we get the exact dimensions then just use that
		if desiredWidth == size.Width && desiredHeight == size.Height {
			return size.Width, size.Height, desiredMethod, nil
		}

		// If we come across a size that's smaller, try and use that
		if desiredWidth < size.Width && desiredHeight < size.Height {
			// Only use our new found size if it's smaller than one we've already picked
			if !foundSize || (targetWidth > size.Width && targetHeight > size.Height) {
				targetWidth = size.Width
				targetHeight = size.Height
				foundSize = true
			}
		}
	}

	// Use the largest dimensions available if we didn't find anything
	if !foundSize {
		targetWidth = largestWidth
		targetHeight = largestHeight
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
