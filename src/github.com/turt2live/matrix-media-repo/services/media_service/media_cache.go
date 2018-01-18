package media_service

import (
	"bytes"
	"container/list"
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/download_tracker"
	"github.com/turt2live/matrix-media-repo/util/errs"
)

// TODO:
// * Eviction timeouts
// * Thumbnails should go through this too (or new cache?)

type StreamedMedia struct {
	Media  *types.Media
	Stream io.ReadCloser
}

type cachedMedia struct {
	media    *types.Media
	contents *bytes.Buffer
}

type mediaCache struct {
	cache   *cache.Cache
	tracker *download_tracker.DownloadTracker
	size    int64
}

type mediaCacheContext struct {
	cache   *cache.Cache
	tracker *download_tracker.DownloadTracker
	size    int64
	svc     *mediaService
	log     *logrus.Entry
	ctx     context.Context
}

var cacheInstance *mediaCache
var cacheSingletonLock = &sync.Once{}

func getBaseCache() (*mediaCache) {
	if cacheInstance == nil {
		cacheSingletonLock.Do(func() {
			if !config.Get().Downloads.Cache.Enabled {
				logrus.Info("Cache is disabled - using dummy instance")
				cacheInstance = &mediaCache{}
			} else {
				logrus.Info("Cache is enabled - setting up")
				trackedMinutes := time.Duration(config.Get().Downloads.Cache.TrackedMinutes) * time.Minute
				cacheInstance = &mediaCache{
					size:    0,
					cache:   cache.New(trackedMinutes, trackedMinutes*2),
					tracker: download_tracker.New(config.Get().Downloads.Cache.TrackedMinutes),
				}
			}
		})
	}

	return cacheInstance
}

func getMediaCache(svc *mediaService) (*mediaCacheContext) {
	return &mediaCacheContext{
		cache:   getBaseCache().cache,
		size:    getBaseCache().size,
		tracker: getBaseCache().tracker,
		svc:     svc,
		log:     svc.log,
		ctx:     svc.ctx,
	}
}

func (c *mediaCacheContext) getCacheKey(server string, mediaId string) string {
	return server + "/" + mediaId
}

func (c *mediaCacheContext) GetMedia(server string, mediaId string) (*StreamedMedia, error) {
	// First see if we have the media in cache
	if config.Get().Downloads.Cache.Enabled {
		item, found := c.cache.Get(c.getCacheKey(server, mediaId))
		if found {
			c.log.Info("Using cached media")
			m := item.(*cachedMedia)
			c.incrementDownloads(m.media)
			c.updateMediaInCache(m.media) // will expire the media if it needs it
			return &StreamedMedia{m.media, util.GetStreamFromBuffer(m.contents)}, nil
		}
	}

	// We proxy the call for media, so we'll at least try and get it first
	c.log.Info("Searching for media")
	media, err := c.svc.store.Get(server, mediaId)
	if err != nil {
		if err == sql.ErrNoRows {
			if util.IsServerOurs(server) {
				c.log.Warn("Media not found")
				return nil, errs.ErrMediaNotFound
			}
		}

		media, err = c.svc.downloadRemoteMedia(server, mediaId)
		if err != nil {
			return nil, err
		}
	}

	exists, err := util.FileExists(media.Location)
	if !exists || err != nil {
		if util.IsServerOurs(server) {
			c.log.Error("Media not found in file store when we expected it to")
			return nil, errs.ErrMediaNotFound
		} else {
			c.log.Warn("Media appears to have been deleted - redownloading")

			media, err = c.svc.downloadRemoteMedia(server, mediaId)
			if err != nil {
				return nil, err
			}
		}
	}

	// At this point we should have a real media object to use, so let's try caching it
	c.incrementDownloads(media)
	cachedMedia, err := c.updateMediaInCache(media)
	if err != nil {
		return nil, err
	}

	if cachedMedia != nil {
		c.log.Info("Using newly cached media")
		return &StreamedMedia{media, util.GetStreamFromBuffer(cachedMedia.contents)}, nil
	}

	c.log.Info("Using media from disk")
	stream, err := os.Open(media.Location)
	if err != nil {
		return nil, err
	}

	return &StreamedMedia{media, stream}, nil
}

func (c *mediaCacheContext) incrementDownloads(media *types.Media) {
	if !config.Get().Downloads.Cache.Enabled {
		return // Not enabled - don't bother
	}

	c.tracker.Increment(media.Origin, media.MediaId)
}

func (c *mediaCacheContext) updateMediaInCache(media *types.Media) (*cachedMedia, error) {
	if !config.Get().Downloads.Cache.Enabled {
		return nil, nil // Not enabled - don't bother (not cached)
	}

	log := c.log.WithFields(logrus.Fields{
		"cache_origin":    media.Origin,
		"cache_mediaId":   media.MediaId,
		"cache_mediaSize": media.SizeBytes,
	})

	downloads := c.tracker.NumDownloads(media.Origin, media.MediaId)
	enoughDownloads := downloads >= config.Get().Downloads.Cache.MinDownloads
	item, found := c.cache.Get(c.getCacheKey(media.Origin, media.MediaId))

	// No longer eligible for the cache - delete item
	// The cached bytes will leave memory over time
	if found && !enoughDownloads {
		log.Info("Removing media from cache because it does not have enough downloads")
		c.cache.Delete(c.getCacheKey(media.Origin, media.MediaId))
		return nil, nil
	}

	// Eligible for the cache, but not in it currently
	if !found && enoughDownloads {
		maxSpace := config.Get().Downloads.Cache.MaxSizeBytes
		usedSpace := c.size
		freeSpace := maxSpace - usedSpace
		mediaSize := media.SizeBytes

		if freeSpace >= mediaSize {
			// Perfect! It'll fit - just cache it
			log.Info("Caching media in memory")
			getBaseCache().size = usedSpace + mediaSize
			return c.cacheMedia(media)
		}

		// We need to clean up some space
		neededSize := (usedSpace + mediaSize) - maxSpace
		log.Info(fmt.Sprintf("Attempting to clear %d bytes from media cache", neededSize))
		clearedSpace := c.clearSpace(neededSize, downloads, mediaSize)
		log.Info(fmt.Sprintf("Cleared %d bytes from media cache", clearedSpace))
		freeSpace += clearedSpace
		if freeSpace >= mediaSize {
			// Now it'll fit - cache it
			log.Info("Caching media in memory")
			getBaseCache().size = usedSpace + mediaSize
			return c.cacheMedia(media)
		}

		log.Warn("Unable to clear enough space for media to be cached")
		return nil, nil
	}

	// At this point the media is already in the correct state (cached and should be or not cached and shouldn't be)
	if found {
		return item.(*cachedMedia), nil
	}
	return nil, nil
}

func (c *mediaCacheContext) cacheMedia(media *types.Media) (*cachedMedia, error) {
	data, err := ioutil.ReadFile(media.Location)
	if err != nil {
		return nil, err
	}

	record := &cachedMedia{media: media, contents: bytes.NewBuffer(data)}
	c.cache.Set(c.getCacheKey(media.Origin, media.MediaId), record, cache.DefaultExpiration)
	return record, nil
}

func (c *mediaCacheContext) clearSpace(neededBytes int64, withDownloadsLessThan int, withSizeLessThan int64) int64 {
	keysToClear := list.New()
	var preppedSpace int64 = 0
	for k, item := range c.cache.Items() {
		record := item.Object.(*cachedMedia)
		if int64(record.contents.Len()) >= withSizeLessThan {
			continue // file too large, cannot evict
		}

		downloads := c.tracker.NumDownloads(record.media.Origin, record.media.MediaId)
		if downloads >= withDownloadsLessThan {
			continue // too many downloads, cannot evict
		}

		// Small enough and has an appropriate file size
		preppedSpace += int64(record.contents.Len())
		keysToClear.PushBack(k)
		if preppedSpace >= neededBytes {
			break // cleared enough space - clear it out
		}
	}

	if preppedSpace < neededBytes {
		// not enough space prepared - don't evict anything
		return 0
	}

	for e := keysToClear.Front(); e != nil; e = e.Next() {
		c.cache.Delete(e.Value.(string))
	}

	return preppedSpace
}
