package internal_cache

import (
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
		} else {
			logrus.Warn("Cache is disabled - setting up a dummy instance")
			instance = NewNoopCache()
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
