package main

import (
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/api/_auth_cache"
	"github.com/turt2live/matrix-media-repo/common/globals"
	"github.com/turt2live/matrix-media-repo/common/runtime"
	"github.com/turt2live/matrix-media-repo/internal_cache"
	"github.com/turt2live/matrix-media-repo/metrics"
	"github.com/turt2live/matrix-media-repo/plugins"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/tasks"
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
}

func stopReloads() {
	// send stop signal to reload fns
	globals.WebReloadChan <- false
	globals.MetricsReloadChan <- false
	globals.DatabaseReloadChan <- false
	globals.DatastoresReloadChan <- false
	globals.AccessTokenReloadChan <- false
	globals.RecurringTasksReloadChan <- false
	globals.CacheReplaceChan <- false
	globals.PluginReloadChan <- false
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
				storage.ReloadDatabase()
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
				internal_cache.ReplaceInstance()
			} else {
				internal_cache.Get().Stop()
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
			}
		}
	}()
}
