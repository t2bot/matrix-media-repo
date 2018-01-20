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
	"github.com/turt2live/matrix-media-repo/util"
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
				maxCooldownSec := util.MaxInt(config.Get().Downloads.Cache.MinEvictedTimeSeconds, config.Get().Downloads.Cache.MinCacheTimeSeconds)
				maxCooldown := time.Duration(maxCooldownSec) * time.Second
				cacheInstance = &mediaCacheFactory{
					size:          0,
					cache:         cache.New(trackedMinutes, trackedMinutes*2),
					cooldownCache: cache.New(maxCooldown*2, maxCooldown*2),
					tracker:       download_tracker.New(config.Get().Downloads.Cache.TrackedMinutes),
				}
			}
		})
	}

	return cacheInstance
}

func Create(ctx context.Context, log *logrus.Entry) (*mediaCache) {
	return &mediaCache{
		cache:         getBaseCache().cache,
		cooldownCache: getBaseCache().cooldownCache,
		size:          getBaseCache().size,
		tracker:       getBaseCache().tracker,
		ctx:           ctx,
		log:           log,
	}
}

func (c *mediaCache) updateItemInCache(cacheKey string, recordId string, mediaSize int64, cacheFn func() (*cachedFile, error), log *logrus.Entry) (*cachedFile, error) {
	downloads := c.tracker.NumDownloads(recordId)
	enoughDownloads := downloads >= config.Get().Downloads.Cache.MinDownloads
	canCache := c.canJoinCache(recordId)
	item, found := c.cache.Get(cacheKey)

	// No longer eligible for the cache - delete item
	// The cached bytes will leave memory over time
	if found && !enoughDownloads {
		log.Info("Removing media from cache because it does not have enough downloads")
		c.cache.Delete(cacheKey)
		c.flagEvicted(recordId)
		return nil, nil
	}

	// Eligible for the cache, but not in it currently (and not on cooldown)
	if !found && enoughDownloads && canCache {
		maxSpace := config.Get().Downloads.Cache.MaxSizeBytes
		usedSpace := c.size
		freeSpace := maxSpace - usedSpace
		mediaSize := mediaSize

		// Don't bother checking for space if it won't fit anyways
		if mediaSize > maxSpace {
			log.Warn("Media too large to cache")
			return nil, nil
		}

		if freeSpace >= mediaSize {
			// Perfect! It'll fit - just cache it
			log.Info("Caching file in memory")
			getBaseCache().size = usedSpace + mediaSize
			c.flagCached(recordId)
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
			c.flagCached(recordId)
			return cacheFn()
		}

		log.Warn("Unable to clear enough space for file to be cached")
		return nil, nil
	}

	// By now the media should be in the correct state (cached or not)
	if found {
		return item.(*cachedFile), nil
	}
	return nil, nil
}

func (c *mediaCache) clearSpace(neededBytes int64, withDownloadsLessThan int, withSizeLessThan int64) int64 {
	type removable struct {
		cacheKey string
		recordId string
	}

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

		if !c.canLeaveCache(recordId) {
			continue // on cooldown, cannot evict
		}

		// Small enough and has an appropriate file size
		preppedSpace += int64(record.contents.Len())
		keysToClear.PushBack(&removable{k, recordId})
		if preppedSpace >= neededBytes {
			break // cleared enough space - clear it out
		}
	}

	if preppedSpace < neededBytes {
		// not enough space prepared - don't evict anything
		return 0
	}

	for e := keysToClear.Front(); e != nil; e = e.Next() {
		toRemove := e.Value.(*removable)
		c.cache.Delete(toRemove.cacheKey)
		c.flagEvicted(toRemove.recordId)
	}

	return preppedSpace
}

func (c *mediaCache) canJoinCache(recordId string) bool {
	item, found := c.cooldownCache.Get(recordId)
	if !found {
		return true // No cooldown means we're probably fine
	}

	cd := item.(*cooldown)
	if !cd.isEviction {
		return true // It should already be in the cache anyways
	}

	return c.checkExpiration(cd, recordId)
}

func (c *mediaCache) canLeaveCache(recordId string) bool {
	item, found := c.cooldownCache.Get(recordId)
	if !found {
		return true // No cooldown means we're probably fine
	}

	cd := item.(*cooldown)
	if cd.isEviction {
		return true // It should already be outside the cache anyways
	}

	return c.checkExpiration(cd, recordId)
}

func (c *mediaCache) checkExpiration(cd *cooldown, recordId string) bool {
	cdType := "Joined cache"
	if cd.isEviction {
		cdType = "Eviction"
	}

	expired := cd.IsExpired()
	if expired {
		c.log.Info(cdType + " cooldown for " + recordId + " has expired")
		c.cooldownCache.Delete(recordId) // cleanup
		return true
	}

	c.log.Warn(cdType + " cooldown on " + recordId + " is still active")
	return false
}

func (c *mediaCache) flagEvicted(recordId string) {
	c.log.Info("Flagging " + recordId + " as evicted (overwriting any previous cooldowns)")
	duration := int64(config.Get().Downloads.Cache.MinEvictedTimeSeconds) * 1000
	c.cooldownCache.Set(recordId, &cooldown{isEviction: true, expiresTs: duration}, cache.DefaultExpiration)
}

func (c *mediaCache) flagCached(recordId string) {
	c.log.Info("Flagging " + recordId + " as joining the cache (overwriting any previous cooldowns)")
	duration := int64(config.Get().Downloads.Cache.MinCacheTimeSeconds) * 1000
	c.cooldownCache.Set(recordId, &cooldown{isEviction: false, expiresTs: duration}, cache.DefaultExpiration)
}
