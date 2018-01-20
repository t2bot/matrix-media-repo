package media_cache

import (
	"container/list"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/util/download_tracker"
)

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

func (c *mediaCache) updateItemInCache(cacheKey string, recordId string, mediaSize int64, cacheFn func() (*cachedFile, error), log *logrus.Entry) (*cachedFile, error) {
	downloads := c.tracker.NumDownloads(recordId)
	enoughDownloads := downloads >= config.Get().Downloads.Cache.MinDownloads
	item, found := c.cache.Get(cacheKey)

	// No longer eligible for the cache - delete item
	// The cached bytes will leave memory over time
	if found && !enoughDownloads {
		log.Info("Removing media from cache because it does not have enough downloads")
		c.cache.Delete(cacheKey)
		return nil, nil
	}

	// Eligible for the cache, but not in it currently
	if !found && enoughDownloads {
		maxSpace := config.Get().Downloads.Cache.MaxSizeBytes
		usedSpace := c.size
		freeSpace := maxSpace - usedSpace
		mediaSize := mediaSize

		if freeSpace >= mediaSize {
			// Perfect! It'll fit - just cache it
			log.Info("Caching file in memory")
			getBaseCache().size = usedSpace + mediaSize
			return cacheFn()
		}

		// We need to clean up some space
		neededSize := (usedSpace + mediaSize) - maxSpace
		log.Info(fmt.Sprintf("Attempting to clear %d bytes from media cache", neededSize))
		clearedSpace := c.clearSpace(neededSize, downloads, mediaSize)
		log.Info(fmt.Sprintf("Cleared %d bytes from media cache", clearedSpace))
		freeSpace += clearedSpace
		if freeSpace >= mediaSize {
			// Now it'll fit - cache it
			log.Info("Caching file in memory")
			getBaseCache().size = usedSpace + mediaSize
			return cacheFn()
		}

		log.Warn("Unable to clear enough space for file to be cached")
		return nil, nil
	}

	// At this point the media is already in the correct state (cached and should be or not cached and shouldn't be)
	if found {
		return item.(*cachedFile), nil
	}
	return nil, nil
}

func (c *mediaCache) clearSpace(neededBytes int64, withDownloadsLessThan int, withSizeLessThan int64) int64 {
	keysToClear := list.New()
	var preppedSpace int64 = 0
	for k, item := range c.cache.Items() {
		record := item.Object.(*cachedFile)
		if int64(record.contents.Len()) >= withSizeLessThan {
			continue // file too large, cannot evict
		}

		recordId := c.getMediaRecordId(record.media)
		if record.thumbnail != nil {
			recordId = c.getThumbnailRecordId(record.thumbnail)
		}

		downloads := c.tracker.NumDownloads(recordId)
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
