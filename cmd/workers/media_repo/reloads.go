package main

import (
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/api"
	"github.com/t2bot/matrix-media-repo/api/_auth_cache"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/globals"
	"github.com/t2bot/matrix-media-repo/common/runtime"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/errcache"
	"github.com/t2bot/matrix-media-repo/limits"
	"github.com/t2bot/matrix-media-repo/matrix"
	"github.com/t2bot/matrix-media-repo/metrics"
	"github.com/t2bot/matrix-media-repo/pgo_internal"
	"github.com/t2bot/matrix-media-repo/plugins"
	"github.com/t2bot/matrix-media-repo/pool"
	"github.com/t2bot/matrix-media-repo/redislib"
	"github.com/t2bot/matrix-media-repo/tasks"
)

func setupReloads() {
	reloadWebOnChan(globals.WebReloadChan)
	reloadMetricsOnChan(globals.MetricsReloadChan)
	reloadDatabaseOnChan(globals.DatabaseReloadChan)
	reloadDatastoresOnChan(globals.DatastoresReloadChan)
	reloadRecurringTasksOnChan(globals.RecurringTasksReloadChan)
	reloadAccessTokensOnChan(globals.AccessTokenReloadChan)
	reloadCacheOnChan(globals.CacheReplaceChan)
	reloadPluginsOnChan(globals.PluginReloadChan)
	reloadPoolOnChan(globals.PoolReloadChan)
	reloadErrorCachesOnChan(globals.ErrorCacheReloadChan)
	reloadMatrixCachesOnChan(globals.MatrixCachesReloadChan)
	reloadPGOOnChan(globals.PGOReloadChan)
	reloadBucketsOnChan(globals.BucketsReloadChan)
}

func stopReloads() {
	// send stop signal to reload fns
	logrus.Debug("Stopping WebReloadChan")
	globals.WebReloadChan <- false
	logrus.Debug("Stopping MetricsReloadChan")
	globals.MetricsReloadChan <- false
	logrus.Debug("Stopping DatabaseReloadChan")
	globals.DatabaseReloadChan <- false
	logrus.Debug("Stopping DatastoresReloadChan")
	globals.DatastoresReloadChan <- false
	logrus.Debug("Stopping AccessTokenReloadChan")
	globals.AccessTokenReloadChan <- false
	logrus.Debug("Stopping RecurringTasksReloadChan")
	globals.RecurringTasksReloadChan <- false
	logrus.Debug("Stopping CacheReplaceChan")
	globals.CacheReplaceChan <- false
	logrus.Debug("Stopping PluginReloadChan")
	globals.PluginReloadChan <- false
	logrus.Debug("Stopping PoolReloadChan")
	globals.PoolReloadChan <- false
	logrus.Debug("Stopping ErrorCacheReloadChan")
	globals.ErrorCacheReloadChan <- false
	logrus.Debug("Stopping MatrixCachesReloadChan")
	globals.MatrixCachesReloadChan <- false
	logrus.Debug("Stopping PGOReloadChan")
	globals.PGOReloadChan <- false
	logrus.Debug("Stopping BucketsReloadChan")
	globals.BucketsReloadChan <- false
}

func reloadWebOnChan(reloadChan chan bool) {
	go func() {
		defer close(reloadChan)
		for {
			shouldReload := <-reloadChan
			if shouldReload {
				api.Reload()
			} else {
				return // received stop
			}
		}
	}()
}

func reloadMetricsOnChan(reloadChan chan bool) {
	go func() {
		defer close(reloadChan)
		for {
			shouldReload := <-reloadChan
			if shouldReload {
				metrics.Reload()
			} else {
				return // received stop
			}
		}
	}()
}

func reloadDatabaseOnChan(reloadChan chan bool) {
	go func() {
		defer close(reloadChan)
		for {
			shouldReload := <-reloadChan
			if shouldReload {
				database.Reload()
				runtime.LoadDatabase()
				globals.DatastoresReloadChan <- true
			} else {
				return // received stop
			}
		}
	}()
}

func reloadDatastoresOnChan(reloadChan chan bool) {
	go func() {
		defer close(reloadChan)
		for {
			shouldReload := <-reloadChan
			if shouldReload {
				runtime.LoadDatastores()
			} else {
				return // received stop
			}
		}
	}()
}

func reloadRecurringTasksOnChan(reloadChan chan bool) {
	go func() {
		defer close(reloadChan)
		for {
			shouldReload := <-reloadChan
			if shouldReload {
				tasks.StopAll()
				tasks.StartAll()
			} else {
				return // received stop
			}
		}
	}()
}

func reloadAccessTokensOnChan(reloadChan chan bool) {
	go func() {
		defer close(reloadChan)
		for {
			shouldReload := <-reloadChan
			if shouldReload {
				_auth_cache.FlushCache()
			} else {
				return // received stop
			}
		}
	}()
}

func reloadCacheOnChan(reloadChan chan bool) {
	go func() {
		defer close(reloadChan)
		for {
			shouldReload := <-reloadChan
			if shouldReload {
				redislib.Reconnect()
			} else {
				redislib.Stop()
				return // received stop
			}
		}
	}()
}

func reloadPluginsOnChan(reloadChan chan bool) {
	go func() {
		defer close(reloadChan)
		for {
			shouldReload := <-reloadChan
			if shouldReload {
				plugins.ReloadPlugins()
			} else {
				plugins.StopPlugins()
				return // received stop
			}
		}
	}()
}

func reloadPoolOnChan(reloadChan chan bool) {
	go func() {
		defer close(reloadChan)
		for {
			shouldReload := <-reloadChan
			if shouldReload {
				pool.AdjustSize()
			} else {
				pool.Drain()
				return // received stop
			}
		}
	}()
}

func reloadErrorCachesOnChan(reloadChan chan bool) {
	go func() {
		defer close(reloadChan)
		for {
			shouldReload := <-reloadChan
			if shouldReload {
				errcache.AdjustSize()
			} else {
				return // received stop
			}
		}
	}()
}

func reloadMatrixCachesOnChan(reloadChan chan bool) {
	go func() {
		defer close(reloadChan)
		for {
			shouldReload := <-reloadChan
			if shouldReload {
				matrix.FlushSigningKeyCache()
			} else {
				return // received stop
			}
		}
	}()
}

func reloadPGOOnChan(reloadChan chan bool) {
	go func() {
		defer close(reloadChan)
		for {
			shouldReload := <-reloadChan
			if shouldReload {
				if config.Get().PGO.Enabled {
					pgo_internal.Enable(config.Get().PGO.SubmitUrl, config.Get().PGO.SubmitKey)
				} else {
					pgo_internal.Disable()
				}
			} else {
				return // received stop
			}
		}
	}()
}

func reloadBucketsOnChan(reloadChan chan bool) {
	go func() {
		defer close(reloadChan)
		for {
			shouldReload := <-reloadChan
			if shouldReload {
				limits.ExpandBuckets()
			} else {
				return // received stop
			}
		}
	}()
}
