package config

import (
	"time"

	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"
	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common/globals"
)

func Watch() *fsnotify.Watcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		sentry.CaptureException(err)
		logrus.Fatal(err)
	}

	err = watcher.Add(Path)
	if err != nil {
		sentry.CaptureException(err)
		logrus.Fatal(err)
	}

	go func() {
		debounced := debounce.New(1 * time.Second)
		for {
			select {
			case _, ok := <-watcher.Events:
				if !ok {
					return
				}
				debounced(onFileChanged)
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logrus.Error("error in config watcher:", err)
			}
		}
	}()

	return watcher
}

func onFileChanged() {
	logrus.Info("Config file change detected - reloading")
	configNow := Get()
	configNew, domainsNew, err := reloadConfig()
	if err != nil {
		logrus.Error("Error reloading configuration - ignoring")
		logrus.Error(err)
		sentry.CaptureException(err)
		return
	}

	logrus.Info("Applying reloaded config live")
	instance = configNew
	domains = domainsNew
	PrintDomainInfo()
	CheckDeprecations()

	logrus.Info("Reloading pool & error cache configurations")
	globals.PoolReloadChan <- true
	globals.ErrorCacheReloadChan <- true

	logrus.Info("Reloading matrix caches")
	globals.MatrixCachesReloadChan <- true

	bindAddressChange := configNew.General.BindAddress != configNow.General.BindAddress
	bindPortChange := configNew.General.Port != configNow.General.Port
	forwardAddressChange := configNew.General.TrustAnyForward != configNow.General.TrustAnyForward
	forwardedHostChange := configNew.General.UseForwardedHost != configNow.General.UseForwardedHost
	if bindAddressChange || bindPortChange || forwardAddressChange || forwardedHostChange {
		logrus.Warn("Webserver configuration changed - remounting")
		globals.WebReloadChan <- true
	}

	metricsEnableChange := configNew.Metrics.Enabled != configNow.Metrics.Enabled
	metricsBindAddressChange := configNew.Metrics.BindAddress != configNow.Metrics.BindAddress
	metricsBindPortChange := configNew.Metrics.Port != configNow.Metrics.Port
	if metricsEnableChange || metricsBindAddressChange || metricsBindPortChange {
		logrus.Warn("Metrics configuration changed - remounting")
		globals.MetricsReloadChan <- true
	}

	databaseChange := configNew.Database.Postgres != configNow.Database.Postgres
	poolConnsChange := configNew.Database.Pool.MaxConnections != configNow.Database.Pool.MaxConnections
	poolIdleChange := configNew.Database.Pool.MaxIdle != configNow.Database.Pool.MaxIdle
	if databaseChange || poolConnsChange || poolIdleChange {
		logrus.Warn("Database configuration changed - reconnecting")
		globals.DatabaseReloadChan <- true
	}

	logChange := configNew.General.LogDirectory != configNow.General.LogDirectory
	if logChange {
		logrus.Warn("Log configuration changed - restart the media repo to apply changes")
	}

	redisEnabledChange := configNew.Redis.Enabled != configNow.Redis.Enabled
	redisShardsChange := hasRedisShardConfigChanged(configNew, configNow)
	if redisEnabledChange || redisShardsChange {
		logrus.Warn("Cache configuration changed - reloading")
		globals.CacheReplaceChan <- true
	}

	// Always expand buckets (could be a no-op)
	globals.BucketsReloadChan <- true

	// Always flush the access token cache
	logrus.Warn("Flushing access token cache")
	globals.AccessTokenReloadChan <- true

	// Always update the datastores
	logrus.Warn("Updating datastores to ensure accuracy")
	globals.DatastoresReloadChan <- true

	logrus.Info("Reloading all plugins")
	globals.PluginReloadChan <- true

	logrus.Info("Restarting recurring tasks")
	globals.RecurringTasksReloadChan <- true

	pgoEnableChange := configNew.PGO.Enabled != configNow.PGO.Enabled
	pgoUrlChange := configNew.PGO.SubmitUrl != configNow.PGO.SubmitUrl
	pgoKeyChange := configNew.PGO.SubmitKey != configNow.PGO.SubmitKey
	if pgoEnableChange || pgoUrlChange || pgoKeyChange {
		logrus.Warn("PGO config changed - reloading")
		globals.PGOReloadChan <- true
	}
}

func hasRedisShardConfigChanged(configNew *MainRepoConfig, configNow *MainRepoConfig) bool {
	oldShards := configNow.Redis.Shards
	newShards := configNew.Redis.Shards
	if len(oldShards) != len(newShards) {
		return true
	}

	for _, s1 := range oldShards {
		has := false
		for _, s2 := range newShards {
			if s1.Name == s2.Name && s1.Address == s2.Address {
				has = true
				break
			}
		}
		if !has {
			return true
		}
	}

	return false
}
