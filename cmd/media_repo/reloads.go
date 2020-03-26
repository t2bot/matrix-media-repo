package main

import (
	"github.com/turt2live/matrix-media-repo/api/webserver"
	"github.com/turt2live/matrix-media-repo/common/globals"
	"github.com/turt2live/matrix-media-repo/common/runtime"
	"github.com/turt2live/matrix-media-repo/ipfs_proxy"
	"github.com/turt2live/matrix-media-repo/metrics"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/tasks"
)

func setupReloads() {
	reloadWebOnChan(globals.WebReloadChan)
	reloadMetricsOnChan(globals.MetricsReloadChan)
	reloadDatabaseOnChan(globals.DatabaseReloadChan)
	reloadDatastoresOnChan(globals.DatastoresReloadChan)
	reloadRecurringTasksOnChan(globals.RecurringTasksReloadChan)
	reloadIpfsOnChan(globals.IPFSReloadChan)
}

func stopReloads() {
	// send stop signal to reload fns
	globals.WebReloadChan <- false
	globals.MetricsReloadChan <- false
	globals.DatabaseReloadChan <- false
	globals.DatastoresReloadChan <- false
	globals.RecurringTasksReloadChan <- false
	globals.IPFSReloadChan <- false
}

func reloadWebOnChan(reloadChan chan bool) {
	go func() {
		defer close(reloadChan)
		for {
			shouldReload := <-reloadChan
			if shouldReload {
				webserver.Reload()
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

func reloadIpfsOnChan(reloadChan chan bool) {
	go func() {
		defer close(reloadChan)
		for {
			shouldReload := <-reloadChan
			if shouldReload {
				ipfs_proxy.Reload()
			} else {
				ipfs_proxy.Stop()
			}
		}
	}()
}
