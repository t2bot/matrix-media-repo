package internal_cache

import (
	"bytes"
	"container/list"
	"fmt"
	"io/ioutil"
	"math"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/metrics"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
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
	cleanupTimer  *time.Ticker
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
var rwLock = &sync.RWMutex{}

func Get() *MediaCache {
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
				cache:         cache.New(trackedMinutes, -1), // we manually clear the cache, so no need for an expiration timer
				cooldownCache: cache.New(maxCooldown*2, maxCooldown*2),
				tracker:       download_tracker.New(config.Get().Downloads.Cache.TrackedMinutes),
				cleanupTimer:  time.NewTicker(5 * time.Minute),
			}
			instance.cache.OnEvicted(func(recordId string, item interface{}) {
				logrus.Infof("Record %s has been evicted from the cache", recordId)
			})

			metrics.OnBeforeMetricsRequested(func() {
				metrics.CacheLiveNumBytes.With(prometheus.Labels{"cache": "media"}).Set(float64(instance.getUnderlyingUsedBytes()))
				metrics.CacheNumLiveItems.With(prometheus.Labels{"cache": "media"}).Set(float64(instance.getUnderlyingItemCount()))
			})

			go func() {
				rctx := rcontext.Initial().LogWithFields(logrus.Fields{"task": "cache_cleanup"})
				for _ = range instance.cleanupTimer.C {
					rctx.Log.Info("Cache cleanup timer fired")
					maxSize := config.Get().Downloads.Cache.MaxSizeBytes

					b := instance.clearSpace(maxSize, math.MaxInt32, maxSize, true, rctx)
					rctx.Log.Infof("Cleared %d bytes from cache during cleanup (%d bytes remain)", b, instance.size)
				}
			}()
		}
	})

	return instance
}

func (c *MediaCache) Reset() {
	if !c.enabled {
		return
	}

	logrus.Warn("Resetting media cache")
	rwLock.Lock()
	c.cache.Flush()
	c.cooldownCache.Flush()
	c.size = 0
	c.tracker.Reset()
	rwLock.Unlock()
}

func (c *MediaCache) Stop() {
	c.cleanupTimer.Stop()
}

func (c *MediaCache) getUnderlyingUsedBytes() int64 {
	var size int64 = 0
	for _, entry := range c.cache.Items() {
		f := entry.Object.(*cachedFile)
		size += int64(f.Contents.Len())
	}
	return size
}

func (c *MediaCache) getUnderlyingItemCount() int {
	return c.cache.ItemCount()
}

func (c *MediaCache) IncrementDownloads(fileHash string) {
	if !c.enabled {
		return
	}

	logrus.Info("File " + fileHash + " has been downloaded")
	rwLock.Lock()
	c.tracker.Increment(fileHash)
	rwLock.Unlock()
}

func (c *MediaCache) GetMedia(media *types.Media, ctx rcontext.RequestContext) (*cachedFile, error) {
	if !c.enabled {
		metrics.CacheMisses.With(prometheus.Labels{"cache": "media"}).Inc()
		return nil, nil
	}

	cacheFn := func() (*cachedFile, error) {
		mediaStream, err := datastore.DownloadStream(ctx, media.DatastoreId, media.Location)
		if err != nil {
			return nil, err
		}
		data, err := ioutil.ReadAll(mediaStream)
		if err != nil {
			return nil, err
		}
		defer mediaStream.Close()

		return &cachedFile{media: media, Contents: bytes.NewBuffer(data)}, nil
	}

	return c.updateItemInCache(media.Sha256Hash, media.SizeBytes, cacheFn, ctx)
}

func (c *MediaCache) GetThumbnail(thumbnail *types.Thumbnail, ctx rcontext.RequestContext) (*cachedFile, error) {
	if !c.enabled {
		metrics.CacheMisses.With(prometheus.Labels{"cache": "media"}).Inc()
		return nil, nil
	}

	cacheFn := func() (*cachedFile, error) {
		mediaStream, err := datastore.DownloadStream(ctx, thumbnail.DatastoreId, thumbnail.Location)
		if err != nil {
			return nil, err
		}
		data, err := ioutil.ReadAll(mediaStream)
		if err != nil {
			return nil, err
		}
		defer mediaStream.Close()

		return &cachedFile{thumbnail: thumbnail, Contents: bytes.NewBuffer(data)}, nil
	}

	return c.updateItemInCache(thumbnail.Sha256Hash, thumbnail.SizeBytes, cacheFn, ctx)
}

func (c *MediaCache) updateItemInCache(recordId string, mediaSize int64, cacheFn func() (*cachedFile, error), ctx rcontext.RequestContext) (*cachedFile, error) {
	downloads := c.tracker.NumDownloads(recordId)
	enoughDownloads := downloads >= config.Get().Downloads.Cache.MinDownloads
	canCache := c.canJoinCache(recordId)
	item, found := c.cache.Get(recordId)

	// No longer eligible for the cache - delete item
	// The cached bytes will leave memory over time
	if found && !enoughDownloads {
		ctx.Log.Info("Removing media from cache because it does not have enough downloads")
		rwLock.Lock()
		metrics.CacheMisses.With(prometheus.Labels{"cache": "media"}).Inc()
		metrics.CacheEvictions.With(prometheus.Labels{"cache": "media", "reason": "not_enough_downloads"}).Inc()
		c.cache.Delete(recordId)
		c.flagEvicted(recordId)
		rwLock.Unlock()
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
			ctx.Log.Warn("Media too large to cache")
			metrics.CacheMisses.With(prometheus.Labels{"cache": "media"}).Inc()
			return nil, nil
		}

		if freeSpace >= mediaSize {
			// Perfect! It'll fit - just cache it
			ctx.Log.Info("Caching file in memory")

			cachedItem, err := cacheFn()
			if err != nil {
				return nil, err
			}
			rwLock.Lock()
			actualSize := int64(cachedItem.Contents.Len())
			c.size = c.size + actualSize
			if actualSize != mediaSize {
				ctx.Log.Warnf("Media size of %d bytes is not the same as %d bytes which have been cached", mediaSize, actualSize)
			}
			c.flagCached(recordId)
			metrics.CacheNumItems.With(prometheus.Labels{"cache": "media"}).Inc()
			metrics.CacheNumBytes.With(prometheus.Labels{"cache": "media"}).Set(float64(c.size))
			c.cache.Set(recordId, cachedItem, cache.NoExpiration)
			rwLock.Unlock()
		}

		// We need to clean up some space
		maxSizeClear := int64(math.Ceil(float64(mediaSize) * 1.25))
		ctx.Log.Info(fmt.Sprintf("Attempting to clear %d bytes from media cache (max evict size %d bytes)", mediaSize, maxSizeClear))
		clearedSpace := c.clearSpace(mediaSize, downloads, maxSizeClear, false, ctx)
		ctx.Log.Info(fmt.Sprintf("Cleared %d bytes from media cache", clearedSpace))
		freeSpace += clearedSpace
		if freeSpace >= mediaSize {
			// Now it'll fit - cache it
			ctx.Log.Info("Caching file in memory")

			cachedItem, err := cacheFn()
			if err != nil {
				return nil, err
			}
			actualSize := int64(cachedItem.Contents.Len())
			rwLock.Lock()
			c.size = c.size + actualSize
			if actualSize != mediaSize {
				ctx.Log.Warnf("Media size of %d bytes is not the same as %d bytes which have been cached", mediaSize, actualSize)
			}
			c.flagCached(recordId)
			metrics.CacheHits.With(prometheus.Labels{"cache": "media"}).Inc()
			metrics.CacheNumItems.With(prometheus.Labels{"cache": "media"}).Inc()
			metrics.CacheNumBytes.With(prometheus.Labels{"cache": "media"}).Set(float64(c.size))
			c.cache.Set(recordId, cachedItem, cache.NoExpiration)
			rwLock.Unlock()

			// This should never happen, but we'll be aggressive in how we handle it.
			if c.size > maxSpace {
				ctx.Log.Warnf("Cache size of %d bytes is larger than prescribed maximum of %d bytes")
				overage := c.size - maxSpace

				// We want to aggressively clear the cache by basically deleting anything that
				// will get us back under the limit. To do this we set the 'safe to clear' download
				// counter at 4x the configured minimum which should catch most things. We also
				// set the maximum file size that can be cleared to the size of the cache which
				// essentially allows us to remove anything.
				downloadsLessThan := config.Get().Downloads.Cache.MinDownloads * 4
				overageCleared := c.clearSpace(overage, downloadsLessThan, maxSpace, true, ctx) // metrics handled internally
				ctx.Log.Infof("Cleared %d bytes from media cache", overageCleared)
			}

			return cachedItem, nil
		}

		ctx.Log.Warn("Unable to clear enough space for file to be cached")
		return nil, nil
	}

	// By now the media should be in the correct state (cached or not)
	if found {
		metrics.CacheHits.With(prometheus.Labels{"cache": "media"}).Inc()
		return item.(*cachedFile), nil
	}
	metrics.CacheMisses.With(prometheus.Labels{"cache": "media"}).Inc()
	return nil, nil
}

func (c *MediaCache) clearSpace(neededBytes int64, withDownloadsLessThan int, withSizeLessThan int64, deleteEvenIfNotEnough bool, ctx rcontext.RequestContext) int64 {
	// This should never happen, but we'll protect against it anyways. If we clear negative space we
	// end up assuming that a very small amount being cleared is enough space for the file we're about
	// to put in, which results in the cache growing beyond the file size limit.
	if neededBytes < 0 {
		ctx.Log.Warnf("Refusing to clear negative space in the cache. Args: neededBytes=%d, withDownloadsLessThan=%d, withSizeLessThan=%d", neededBytes, withDownloadsLessThan, withSizeLessThan)
		return 0
	}

	type removable struct {
		cacheKey string
		recordId string
	}

	keysToClear := list.New()
	var preppedSpace int64 = 0
	for k, item := range c.cache.Items() {
		record := item.Object.(*cachedFile)

		var recordId string
		if record.thumbnail != nil {
			recordId = record.thumbnail.Sha256Hash
		} else {
			recordId = record.media.Sha256Hash
		}

		if int64(record.Contents.Len()) >= withSizeLessThan {
			continue // file too large, cannot evict
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

	if preppedSpace < neededBytes && !deleteEvenIfNotEnough {
		// not enough space prepared - don't evict anything
		return 0
	}

	rwLock.Lock()

	for e := keysToClear.Front(); e != nil; e = e.Next() {
		toRemove := e.Value.(*removable)
		c.cache.Delete(toRemove.cacheKey)
		c.flagEvicted(toRemove.recordId)
		metrics.CacheEvictions.With(prometheus.Labels{"cache": "media", "reason": "need_space"}).Inc()
		metrics.CacheNumItems.With(prometheus.Labels{"cache": "media"}).Dec()
	}

	c.size -= preppedSpace
	metrics.CacheNumBytes.With(prometheus.Labels{"cache": "media"}).Set(float64(c.size))

	rwLock.Unlock()

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
	expireTs := (int64(config.Get().Downloads.Cache.MinEvictedTimeSeconds) * 1000) + util.NowMillis()
	c.cooldownCache.Set(recordId, &cooldown{isEviction: true, expiresTs: expireTs}, cache.DefaultExpiration)
}

func (c *MediaCache) flagCached(recordId string) {
	logrus.Info("Flagging " + recordId + " as joining the cache (overwriting any previous cooldowns)")
	expireTs := (int64(config.Get().Downloads.Cache.MinCacheTimeSeconds) * 1000) + util.NowMillis()
	c.cooldownCache.Set(recordId, &cooldown{isEviction: false, expiresTs: expireTs}, cache.DefaultExpiration)
}

func (c *cooldown) IsExpired() bool {
	return util.NowMillis() >= c.expiresTs
}
