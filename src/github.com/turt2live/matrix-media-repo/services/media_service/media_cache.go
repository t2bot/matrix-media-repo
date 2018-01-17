package media_service

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/errs"
)

// TODO:
// * Buckets (timeline queue)
// * Media cache
// * Ordered set for sizes (eviction)

type bucket struct {
	minuteTs  int64
	downloads int
}

type cachedMedia struct {
	buckets        []*bucket
	totalDownloads int
	media          *types.Media
}

type mediaCache struct {
	cache *cache.Cache
	size  int64
}

type mediaCacheContext struct {
	cache *cache.Cache
	size  int64
	svc   *mediaService
	log   *logrus.Entry
	ctx   context.Context
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
					size:  0,
					cache: cache.New(trackedMinutes, trackedMinutes*2),
				}
			}
		})
	}

	return cacheInstance
}

func getMediaCache(svc *mediaService) (*mediaCacheContext) {
	return &mediaCacheContext{
		cache: getBaseCache().cache,
		size:  getBaseCache().size,
		svc:   svc,
		log:   svc.log,
		ctx:   svc.ctx,
	}
}

func (c *mediaCacheContext) GetMedia(server string, mediaId string) (*types.Media, error) {
	// First see if we have the media in cache
	if config.Get().Downloads.Cache.Enabled {
		item, found := c.cache.Get(server + "/" + mediaId)
		if found {
			c.log.Info("Using cached media")
			c.incrementDownloads(server, mediaId)
			// TODO: Return a stream to make the cache useful
			return item.(*cachedMedia).media, nil
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
	c.incrementDownloads(server, mediaId)
	// TODO: Cache media

	return media, nil
}

func (c *mediaCacheContext) incrementDownloads(server string, mediaId string) {
	if !config.Get().Downloads.Cache.Enabled {
		return // Not enabled - don't bother
	}

	// TODO: Incremement downloads
	// TODO: Actually cache media that hits the minimum (do this somewhere else?)
}
