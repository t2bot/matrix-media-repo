package media_cache

import (
	"bytes"
	"container/list"
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/services/media_service"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/download_tracker"
	"github.com/turt2live/matrix-media-repo/util/errs"
)

// TODO:
// * Eviction timeouts
// * Thumbnails should go through this too (or new cache?)

type cachedMedia struct {
	media    *types.Media
	contents *bytes.Buffer
}

type mediaCacheFactory struct {
	cache   *cache.Cache
	tracker *download_tracker.DownloadTracker
	size    int64
}

type mediaCache struct {
	cache   *cache.Cache
	tracker *download_tracker.DownloadTracker
	size    int64
	log     *logrus.Entry
	ctx     context.Context
}

var cacheInstance *mediaCacheFactory
var cacheSingletonLock = &sync.Once{}

func getBaseCache() (*mediaCacheFactory) {
	if cacheInstance == nil {
		cacheSingletonLock.Do(func() {
			if !config.Get().Downloads.Cache.Enabled {
				logrus.Info("Cache is disabled - using dummy instance")
				cacheInstance = &mediaCacheFactory{}
			} else {
				logrus.Info("Cache is enabled - setting up")
				trackedMinutes := time.Duration(config.Get().Downloads.Cache.TrackedMinutes) * time.Minute
				cacheInstance = &mediaCacheFactory{
					size:    0,
					cache:   cache.New(trackedMinutes, trackedMinutes*2),
					tracker: download_tracker.New(config.Get().Downloads.Cache.TrackedMinutes),
				}
			}
		})
	}

	return cacheInstance
}

func Create(ctx context.Context, log *logrus.Entry) (*mediaCache) {
	return &mediaCache{
		cache:   getBaseCache().cache,
		size:    getBaseCache().size,
		tracker: getBaseCache().tracker,
		ctx:     ctx,
		log:     log,
	}
}

func (c *mediaCache) getKeyForMedia(server string, mediaId string) string {
	return server + "/" + mediaId
}

func (c *mediaCache) GetMedia(server string, mediaId string) (*types.StreamedMedia, error) {
	media, err := c.GetRawMedia(server, mediaId)
	if err != nil {
		return nil, err
	}

	// At this point we should have a real media object to use, so let's try caching it
	c.incrementDownloads(media)
	cachedMedia, err := c.updateMediaInCache(media)
	if err != nil {
		return nil, err
	}

	if cachedMedia != nil {
		c.log.Info("Using newly cached media")
		return &types.StreamedMedia{Media: media, Stream: util.GetStreamFromBuffer(cachedMedia.contents)}, nil
	}

	c.log.Info("Using media from disk")
	stream, err := os.Open(media.Location)
	if err != nil {
		return nil, err
	}

	return &types.StreamedMedia{Media: media, Stream: stream}, nil
}

func (c *mediaCache) GetRawMedia(server string, mediaId string) (*types.Media, error) {
	mediaSvc := media_service.New(c.ctx, c.log)

	// First see if we have the media in cache
	if config.Get().Downloads.Cache.Enabled {
		item, found := c.cache.Get(c.getKeyForMedia(server, mediaId))
		if found {
			c.log.Info("Using cached media")
			m := item.(*cachedMedia)
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
				return nil, errs.ErrMediaNotFound
			}
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
			return nil, errs.ErrMediaNotFound
		} else {
			c.log.Warn("Media appears to have been deleted - redownloading")

			media, err = mediaSvc.GetRemoteMediaDirect(server, mediaId)
			if err != nil {
				return nil, err
			}
		}
	}

	return media, nil
}

func (c *mediaCache) incrementDownloads(media *types.Media) {
	if !config.Get().Downloads.Cache.Enabled {
		return // Not enabled - don't bother
	}

	c.tracker.Increment(media.Origin, media.MediaId)
}

func (c *mediaCache) updateMediaInCache(media *types.Media) (*cachedMedia, error) {
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
	item, found := c.cache.Get(c.getKeyForMedia(media.Origin, media.MediaId))

	// No longer eligible for the cache - delete item
	// The cached bytes will leave memory over time
	if found && !enoughDownloads {
		log.Info("Removing media from cache because it does not have enough downloads")
		c.cache.Delete(c.getKeyForMedia(media.Origin, media.MediaId))
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

func (c *mediaCache) cacheMedia(media *types.Media) (*cachedMedia, error) {
	data, err := ioutil.ReadFile(media.Location)
	if err != nil {
		return nil, err
	}

	record := &cachedMedia{media: media, contents: bytes.NewBuffer(data)}
	c.cache.Set(c.getKeyForMedia(media.Origin, media.MediaId), record, cache.DefaultExpiration)
	return record, nil
}

func (c *mediaCache) clearSpace(neededBytes int64, withDownloadsLessThan int, withSizeLessThan int64) int64 {
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
