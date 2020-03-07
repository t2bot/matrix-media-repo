package config

import (
	"time"

	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/globals"
)

func Watch() *fsnotify.Watcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logrus.Fatal(err)
	}

	err = watcher.Add(Path)
	if err != nil {
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
		return
	}

	logrus.Info("Applying reloaded config live")
	instance = configNew
	domains = domainsNew
	PrintDomainInfo()

	bindAddressChange := configNew.General.BindAddress != configNow.General.BindAddress
	bindPortChange := configNew.General.Port != configNow.General.Port
	forwardAddressChange := configNew.General.TrustAnyForward != configNow.General.TrustAnyForward
	forwardedHostChange := configNew.General.UseForwardedHost != configNow.General.UseForwardedHost
	featureChanged := hasWebFeatureChanged(configNew, configNow)
	if bindAddressChange || bindPortChange || forwardAddressChange || forwardedHostChange || featureChanged {
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

	ipfsDaemonChange := configNew.Features.IPFS.Daemon.Enabled != configNow.Features.IPFS.Daemon.Enabled
	ipfsDaemonPathChange := configNew.Features.IPFS.Daemon.RepoPath != configNow.Features.IPFS.Daemon.RepoPath
	if ipfsDaemonChange || ipfsDaemonPathChange {
		logrus.Warn("IPFS Daemon options changed - reloading")
		globals.IPFSReloadChan <- true
	}

	// Always update the datastores
	logrus.Warn("Updating datastores to ensure accuracy")
	globals.DatastoresReloadChan <- true

	logrus.Info("Restarting recurring tasks")
	globals.RecurringTasksReloadChan <- true
}

func hasWebFeatureChanged(configNew *MainRepoConfig, configNow *MainRepoConfig) bool {
	if configNew.Features.MSC2448Blurhash.Enabled != configNow.Features.MSC2448Blurhash.Enabled {
		return true
	}
	if configNew.Features.IPFS.Enabled != configNow.Features.IPFS.Enabled {
		return true
	}

	return false
}
