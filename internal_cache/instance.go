package internal_cache

import (
	"github.com/getsentry/sentry-go"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
)

var instance ContentCache
var lock = &sync.Once{}

func Get() ContentCache {
	if instance != nil {
		return instance
	}

	lock.Do(func() {
		if config.Get().Redis.Enabled {
			logrus.Info("Setting up Redis cache")
			instance = NewRedisCache(config.Get().Redis)
		} else if config.Get().Features.Redis.Enabled {
			logrus.Info("Setting up Redis cache")

			warnMsg := "Your configuration uses a legacy approach for enabling Redis support. Please move this to the root config or visit #media-repo:t2bot.io for assistance."
			logrus.Warn(warnMsg)
			sentry.CaptureMessage(warnMsg)

			instance = NewRedisCache(config.Get().Features.Redis)
		} else if !config.Get().Downloads.Cache.Enabled {
			logrus.Warn("Cache is disabled - setting up a dummy instance")
			instance = NewNoopCache()
		} else {
			logrus.Info("Setting up in-memory cache")

			warnMsg := "The built-in cache mechanism is being removed in a future version. Please set up Redis as a cache mechanism."
			logrus.Warn(warnMsg)
			sentry.CaptureMessage(warnMsg)

			instance = NewMemoryCache()
		}
	})

	return instance
}

func ReplaceInstance() {
	if instance != nil {
		instance.Reset()
		instance.Stop()
		instance = nil
	}

	Get() // initializes new cache
}
