package internal_cache

import (
	"container/list"
	"fmt"
	"io"
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
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/download_tracker"
	"github.com/turt2live/matrix-media-repo/util/util_byte_seeker"
)

type cooldown struct {
	isEviction bool
	expiresTs  int64
}

type MemoryCache struct {
	cache         *cache.Cache
	cooldownCache *cache.Cache
	tracker       *download_tracker.DownloadTracker
	cleanupTimer  *time.Ticker
	rwLock        *sync.RWMutex
}

func NewMemoryCache() *MemoryCache {
	trackedMinutes := time.Duration(config.Get().Downloads.Cache.TrackedMinutes) * time.Minute
	maxCooldownSec := util.MaxInt(config.Get().Downloads.Cache.MinEvictedTimeSeconds, config.Get().Downloads.Cache.MinCacheTimeSeconds)
	maxCooldown := time.Duration(maxCooldownSec) * time.Second
	memCache := &MemoryCache{
		cache:         cache.New(trackedMinutes, -1), // we manually clear the cache, so no need for an expiration timer
		cooldownCache: cache.New(maxCooldown*2, maxCooldown*2),
		tracker:       download_tracker.New(config.Get().Downloads.Cache.TrackedMinutes),
		cleanupTimer:  time.NewTicker(5 * time.Minute),
		rwLock:        &sync.RWMutex{},
	}

	metrics.OnBeforeMetricsRequested(func() {
		metrics.CacheLiveNumBytes.With(prometheus.Labels{"cache": "media"}).Set(float64(memCache.getUnderlyingUsedBytes()))
		metrics.CacheNumLiveItems.With(prometheus.Labels{"cache": "media"}).Set(float64(memCache.getUnderlyingItemCount()))
		metrics.CacheNumBytes.With(prometheus.Labels{"cache": "media"}).Set(float64(memCache.getUnderlyingUsedBytes()))
		metrics.CacheNumItems.With(prometheus.Labels{"cache": "media"}).Set(float64(memCache.getUnderlyingItemCount()))
	})

	go func() {
		rctx := rcontext.Initial().LogWithFields(logrus.Fields{"task": "cache_cleanup"})
		for _ = range memCache.cleanupTimer.C {
			rctx.Log.Info("Cache cleanup timer fired")
			maxSize := config.Get().Downloads.Cache.MaxSizeBytes

			b := memCache.clearSpace(maxSize, math.MaxInt32, maxSize, true, rctx)
			rctx.Log.Infof("Cleared %d bytes from cache during cleanup (%d bytes remain)", b, memCache.getUnderlyingUsedBytes())
		}
	}()

	return memCache
}

func (c *MemoryCache) Reset() {
	c.rwLock.Lock()
	c.cache.Flush()
	c.cooldownCache.Flush()
	c.tracker.Reset()
	c.rwLock.Unlock()
}

func (c *MemoryCache) Stop() {
	c.cleanupTimer.Stop()
}

func (c *MemoryCache) MarkDownload(fileHash string) {
	logrus.Info("File " + fileHash + " has been downloaded")
	c.rwLock.Lock()
	c.tracker.Increment(fileHash)
	c.rwLock.Unlock()
}

func (c *MemoryCache) GetMedia(sha256hash string, contents FetchFunction, ctx rcontext.RequestContext) (*CachedContent, error) {
	return c.updateItemInCache(sha256hash, contents, ctx)
}

func (c *MemoryCache) UploadMedia(sha256hash string, content io.ReadCloser, ctx rcontext.RequestContext) error {
	// Nothing to do for this cache type
	return nil
}

func (c *MemoryCache) getUnderlyingUsedBytes() int64 {
	var size int64 = 0
	for _, entry := range c.cache.Items() {
		f := entry.Object.([]byte)
		size += int64(len(f))
	}
	return size
}

func (c *MemoryCache) getUnderlyingItemCount() int {
	return c.cache.ItemCount()
}

func (c *MemoryCache) canJoinCache(sha256hash string) bool {
	item, found := c.cooldownCache.Get(sha256hash)
	if !found {
		return true // No cooldown means we're probably fine
	}

	cd := item.(*cooldown)
	if !cd.isEviction {
		return true // It should already be in the cache anyways
	}

	return c.checkExpiration(cd, sha256hash)
}

func (c *MemoryCache) canLeaveCache(sha256hash string) bool {
	item, found := c.cooldownCache.Get(sha256hash)
	if !found {
		return true // No cooldown means we're probably fine
	}

	cd := item.(*cooldown)
	if cd.isEviction {
		return true // It should already be outside the cache anyways
	}

	return c.checkExpiration(cd, sha256hash)
}

func (c *MemoryCache) checkExpiration(cd *cooldown, sha256hash string) bool {
	cdType := "Joined cache"
	if cd.isEviction {
		cdType = "Eviction"
	}

	expired := cd.IsExpired()
	if expired {
		logrus.Info(cdType + " cooldown for " + sha256hash + " has expired")
		c.cooldownCache.Delete(sha256hash) // cleanup
		return true
	}

	logrus.Warn(cdType + " cooldown on " + sha256hash + " is still active")
	return false
}

func (c *MemoryCache) flagEvicted(sha256hash string) {
	logrus.Info("Flagging " + sha256hash + " as evicted (overwriting any previous cooldowns)")
	expireTs := (int64(config.Get().Downloads.Cache.MinEvictedTimeSeconds) * 1000) + util.NowMillis()
	c.cooldownCache.Set(sha256hash, &cooldown{isEviction: true, expiresTs: expireTs}, cache.DefaultExpiration)
}

func (c *MemoryCache) flagCached(sha256hash string) {
	logrus.Info("Flagging " + sha256hash + " as joining the cache (overwriting any previous cooldowns)")
	expireTs := (int64(config.Get().Downloads.Cache.MinCacheTimeSeconds) * 1000) + util.NowMillis()
	c.cooldownCache.Set(sha256hash, &cooldown{isEviction: false, expiresTs: expireTs}, cache.DefaultExpiration)
}

func (c *MemoryCache) updateItemInCache(sha256hash string, fetchFn FetchFunction, ctx rcontext.RequestContext) (*CachedContent, error) {
	downloads := c.tracker.NumDownloads(sha256hash)
	enoughDownloads := downloads >= config.Get().Downloads.Cache.MinDownloads
	canCache := c.canJoinCache(sha256hash)
	item, found := c.cache.Get(sha256hash)

	// No longer eligible for the cache - delete item
	// The cached bytes will leave memory over time
	if found && !enoughDownloads {
		ctx.Log.Info("Removing media from cache because it does not have enough downloads")
		c.rwLock.Lock()
		metrics.CacheMisses.With(prometheus.Labels{"cache": "media"}).Inc()
		metrics.CacheEvictions.With(prometheus.Labels{"cache": "media", "reason": "not_enough_downloads"}).Inc()
		c.cache.Delete(sha256hash)
		c.flagEvicted(sha256hash)
		c.rwLock.Unlock()
		return nil, nil
	}

	// The media is still valid, so return it
	if found {
		metrics.CacheHits.With(prometheus.Labels{"cache": "media"}).Inc()
		return &CachedContent{Contents: util_byte_seeker.NewByteSeeker(item.([]byte))}, nil
	}

	// Eligible for the cache, but not in it currently (and not on cooldown)
	if !found && enoughDownloads && canCache {
		s, err := fetchFn()
		if err != nil {
			return nil, err
		}
		defer s.Close()
		b, err := ioutil.ReadAll(s)
		if err != nil {
			return nil, err
		}

		maxSpace := config.Get().Downloads.Cache.MaxSizeBytes
		usedSpace := c.getUnderlyingUsedBytes()
		freeSpace := maxSpace - usedSpace
		mediaSize := int64(len(b))

		// Don't bother checking for space if it won't fit anyways
		if mediaSize > maxSpace {
			ctx.Log.Warn("Media too large to cache")
			metrics.CacheMisses.With(prometheus.Labels{"cache": "media"}).Inc()
			return nil, nil
		}

		if freeSpace >= mediaSize {
			// Perfect! It'll fit - just cache it
			ctx.Log.Info("Caching file in memory")

			c.rwLock.Lock()
			c.flagCached(sha256hash)
			metrics.CacheHits.With(prometheus.Labels{"cache": "media"}).Inc()
			c.cache.Set(sha256hash, b, cache.NoExpiration)
			c.rwLock.Unlock()
			return &CachedContent{Contents: util_byte_seeker.NewByteSeeker(b)}, nil
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

			c.rwLock.Lock()
			c.flagCached(sha256hash)
			metrics.CacheHits.With(prometheus.Labels{"cache": "media"}).Inc()
			c.cache.Set(sha256hash, b, cache.NoExpiration)
			c.rwLock.Unlock()

			// This should never happen, but we'll be aggressive in how we handle it.
			if c.getUnderlyingUsedBytes() > maxSpace {
				ctx.Log.Warnf("Cache size of %d bytes is larger than prescribed maximum of %d bytes")
				overage := c.getUnderlyingUsedBytes() - maxSpace

				// We want to aggressively clear the cache by basically deleting anything that
				// will get us back under the limit. To do this we set the 'safe to clear' download
				// counter at 4x the configured minimum which should catch most things. We also
				// set the maximum file size that can be cleared to the size of the cache which
				// essentially allows us to remove anything.
				downloadsLessThan := config.Get().Downloads.Cache.MinDownloads * 4
				overageCleared := c.clearSpace(overage, downloadsLessThan, maxSpace, true, ctx) // metrics handled internally
				ctx.Log.Infof("Cleared %d bytes from media cache", overageCleared)
			}

			return &CachedContent{Contents: util_byte_seeker.NewByteSeeker(b)}, nil
		}

		ctx.Log.Warn("Unable to clear enough space for file to be cached")
		return nil, nil
	}

	metrics.CacheMisses.With(prometheus.Labels{"cache": "media"}).Inc()
	return nil, nil
}

func (c *MemoryCache) clearSpace(neededBytes int64, withDownloadsLessThan int, withSizeLessThan int64, deleteEvenIfNotEnough bool, ctx rcontext.RequestContext) int64 {
	// This should never happen, but we'll protect against it anyways. If we clear negative space we
	// end up assuming that a very small amount being cleared is enough space for the file we're about
	// to put in, which results in the cache growing beyond the file size limit.
	if neededBytes < 0 {
		ctx.Log.Warnf("Refusing to clear negative space in the cache. Args: neededBytes=%d, withDownloadsLessThan=%d, withSizeLessThan=%d", neededBytes, withDownloadsLessThan, withSizeLessThan)
		return 0
	}

	type removable struct {
		cacheKey   string
		sha256hash string
	}

	keysToClear := list.New()
	var preppedSpace int64 = 0
	for k, item := range c.cache.Items() {
		b := item.Object.([]byte)

		if int64(len(b)) >= withSizeLessThan {
			continue // file too large, cannot evict
		}

		downloads := c.tracker.NumDownloads(k)
		if downloads >= withDownloadsLessThan {
			continue // too many downloads, cannot evict
		}

		if !c.canLeaveCache(k) {
			continue // on cooldown, cannot evict
		}

		// Small enough and has an appropriate file size
		preppedSpace += int64(len(b))
		keysToClear.PushBack(k)
		if preppedSpace >= neededBytes {
			break // cleared enough space - clear it out
		}
	}

	if preppedSpace < neededBytes && !deleteEvenIfNotEnough {
		// not enough space prepared - don't evict anything
		return 0
	}

	c.rwLock.Lock()

	for e := keysToClear.Front(); e != nil; e = e.Next() {
		toRemove := e.Value.(string)
		c.cache.Delete(toRemove)
		c.flagEvicted(toRemove)
		metrics.CacheEvictions.With(prometheus.Labels{"cache": "media", "reason": "need_space"}).Inc()
	}

	c.rwLock.Unlock()

	return preppedSpace
}

func (c *cooldown) IsExpired() bool {
	return util.NowMillis() >= c.expiresTs
}
