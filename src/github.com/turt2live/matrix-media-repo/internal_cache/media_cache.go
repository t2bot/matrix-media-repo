package internal_cache

import (
	"bytes"
	"container/list"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/download_tracker"
)

type MediaCache struct {
	cache         *cache.Cache
	cooldownCache *cache.Cache
	tracker       *download_tracker.DownloadTracker
	size          int64
	enabled       bool
}

type cachedFile struct {
	media     *types.Media
	thumbnail *types.Thumbnail
	Contents  *bytes.Buffer
}

type cooldown struct {
	isEviction bool
	expiresTs  int64
}

var instance *MediaCache
var lock = &sync.Once{}

func Get() (*MediaCache) {
	if instance != nil {
		return instance
	}

	lock.Do(func() {
		if !config.Get().Downloads.Cache.Enabled {
			logrus.Warn("Cache is disabled - setting up a dummy instance")
			instance = &MediaCache{enabled: false}
		} else {
			logrus.Info("Setting up cache")
			trackedMinutes := time.Duration(config.Get().Downloads.Cache.TrackedMinutes) * time.Minute
			maxCooldownSec := util.MaxInt(config.Get().Downloads.Cache.MinEvictedTimeSeconds, config.Get().Downloads.Cache.MinCacheTimeSeconds)
			maxCooldown := time.Duration(maxCooldownSec) * time.Second
			instance = &MediaCache{
				enabled:       true,
				size:          0,
				cache:         cache.New(trackedMinutes, trackedMinutes*2),
				cooldownCache: cache.New(maxCooldown*2, maxCooldown*2),
				tracker:       download_tracker.New(config.Get().Downloads.Cache.TrackedMinutes),
			}
		}
	})

	return instance
}

func (c *MediaCache) Reset() {
	if !c.enabled {
		return
	}

	logrus.Warn("Resetting media cache")
	c.cache.Flush()
	c.cooldownCache.Flush()
	c.size = 0
	c.tracker.Reset()
}

func (c *MediaCache) IncrementDownloads(fileHash string) {
	if !c.enabled {
		return
	}

	logrus.Info("File " + fileHash + " has been downloaded")
	c.tracker.Increment(fileHash)
}

func (c *MediaCache) GetMedia(media *types.Media, log *logrus.Entry) (*cachedFile, error) {
	if !c.enabled {
		return nil, nil
	}

	cacheFn := func() (*cachedFile, error) {
		data, err := ioutil.ReadFile(media.Location)
		if err != nil {
			return nil, err
		}

		return &cachedFile{media: media, Contents: bytes.NewBuffer(data)}, nil
	}

	return c.updateItemInCache(media.Sha256Hash, media.SizeBytes, cacheFn, log)
}

func (c *MediaCache) GetThumbnail(thumbnail *types.Thumbnail, log *logrus.Entry) (*cachedFile, error) {
	if !c.enabled {
		return nil, nil
	}

	cacheFn := func() (*cachedFile, error) {
		data, err := ioutil.ReadFile(thumbnail.Location)
		if err != nil {
			return nil, err
		}

		return &cachedFile{thumbnail: thumbnail, Contents: bytes.NewBuffer(data)}, nil
	}

	return c.updateItemInCache(*thumbnail.Sha256Hash, thumbnail.SizeBytes, cacheFn, log)
}

func (c *MediaCache) updateItemInCache(recordId string, mediaSize int64, cacheFn func() (*cachedFile, error), log *logrus.Entry) (*cachedFile, error) {
	downloads := c.tracker.NumDownloads(recordId)
	enoughDownloads := downloads >= config.Get().Downloads.Cache.MinDownloads
	canCache := c.canJoinCache(recordId)
	item, found := c.cache.Get(recordId)

	// No longer eligible for the cache - delete item
	// The cached bytes will leave memory over time
	if found && !enoughDownloads {
		log.Info("Removing media from cache because it does not have enough downloads")
		c.cache.Delete(recordId)
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
			c.size = usedSpace + mediaSize
			c.flagCached(recordId)

			cachedItem, err := cacheFn()
			if err != nil {
				return nil, err
			}
			c.cache.Set(recordId, cachedItem, cache.DefaultExpiration)
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
			c.size = usedSpace + mediaSize
			c.flagCached(recordId)

			cachedItem, err := cacheFn()
			if err != nil {
				return nil, err
			}
			c.cache.Set(recordId, cachedItem, cache.DefaultExpiration)
			return cachedItem, nil
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

func (c *MediaCache) clearSpace(neededBytes int64, withDownloadsLessThan int, withSizeLessThan int64) int64 {
	type removable struct {
		cacheKey string
		recordId string
	}

	keysToClear := list.New()
	var preppedSpace int64 = 0
	for k, item := range c.cache.Items() {
		record := item.Object.(*cachedFile)
		if int64(record.Contents.Len()) >= withSizeLessThan {
			continue // file too large, cannot evict
		}

		var recordId string
		if record.thumbnail != nil {
			recordId = *record.thumbnail.Sha256Hash
		} else {
			recordId = record.media.Sha256Hash
		}

		downloads := c.tracker.NumDownloads(recordId)
		if downloads >= withDownloadsLessThan {
			continue // too many downloads, cannot evict
		}

		if !c.canLeaveCache(recordId) {
			continue // on cooldown, cannot evict
		}

		// Small enough and has an appropriate file size
		preppedSpace += int64(record.Contents.Len())
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

func (c *MediaCache) canJoinCache(recordId string) bool {
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

func (c *MediaCache) canLeaveCache(recordId string) bool {
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

func (c *MediaCache) checkExpiration(cd *cooldown, recordId string) bool {
	cdType := "Joined cache"
	if cd.isEviction {
		cdType = "Eviction"
	}

	expired := cd.IsExpired()
	if expired {
		logrus.Info(cdType + " cooldown for " + recordId + " has expired")
		c.cooldownCache.Delete(recordId) // cleanup
		return true
	}

	logrus.Warn(cdType + " cooldown on " + recordId + " is still active")
	return false
}

func (c *MediaCache) flagEvicted(recordId string) {
	logrus.Info("Flagging " + recordId + " as evicted (overwriting any previous cooldowns)")
	duration := int64(config.Get().Downloads.Cache.MinEvictedTimeSeconds) * 1000
	c.cooldownCache.Set(recordId, &cooldown{isEviction: true, expiresTs: duration}, cache.DefaultExpiration)
}

func (c *MediaCache) flagCached(recordId string) {
	logrus.Info("Flagging " + recordId + " as joining the cache (overwriting any previous cooldowns)")
	duration := int64(config.Get().Downloads.Cache.MinCacheTimeSeconds) * 1000
	c.cooldownCache.Set(recordId, &cooldown{isEviction: false, expiresTs: duration}, cache.DefaultExpiration)
}

func (c *cooldown) IsExpired() bool {
	return util.NowMillis() >= c.expiresTs
}
