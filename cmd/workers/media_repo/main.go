package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/api"
	"github.com/t2bot/matrix-media-repo/common/assets"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/logging"
	"github.com/t2bot/matrix-media-repo/common/runtime"
	"github.com/t2bot/matrix-media-repo/common/version"
	"github.com/t2bot/matrix-media-repo/metrics"
	"github.com/t2bot/matrix-media-repo/pgo_internal"
	"github.com/t2bot/matrix-media-repo/tasks"
)

func main() {
	configPath := flag.String("config", "media-repo.yaml", "The path to the configuration")
	migrationsPath := flag.String("migrations", config.DefaultMigrationsPath, "The absolute path for the migrations folder")
	templatesPath := flag.String("templates", config.DefaultTemplatesPath, "The absolute path for the templates folder")
	assetsPath := flag.String("assets", config.DefaultAssetsPath, "The absolute path for the assets folder")
	versionFlag := flag.Bool("version", false, "Prints the version and exits")
	flag.Parse()

	if *versionFlag {
		version.Print(false)
		return // exit 0
	}

	// Override config path with config for Docker users
	configEnv := os.Getenv("REPO_CONFIG")
	if configEnv != "" {
		configPath = &configEnv
	}

	config.Path = *configPath
	if config.Get().Sentry.Enabled {
		logrus.Info("Setting up Sentry for debugging...")
		err := sentry.Init(sentry.ClientOptions{
			Dsn:         config.Get().Sentry.Dsn,
			Environment: config.Get().Sentry.Environment,
			Debug:       config.Get().Sentry.Debug,
			Release:     fmt.Sprintf("%s-%s", version.Version, version.GitCommit),
		})
		if err != nil {
			panic(err)
		}
	}
	defer sentry.Flush(2 * time.Second)
	defer sentry.Recover()

	defer assets.Cleanup()
	assets.SetupMigrations(*migrationsPath)
	assets.SetupTemplates(*templatesPath)
	assets.SetupAssets(*assetsPath)

	err := logging.Setup(
		config.Get().General.LogDirectory,
		config.Get().General.LogColors,
		config.Get().General.JsonLogs,
		config.Get().General.LogLevel,
	)
	if err != nil {
		panic(err)
	}

	logrus.Info("Starting up...")
	runtime.RunStartupSequence()

	logrus.Info("Starting recurring tasks...")
	tasks.StartAll()

	logrus.Info("Starting config watcher...")
	watcher := config.Watch()
	defer func(watcher *fsnotify.Watcher) {
		_ = watcher.Close()
	}(watcher)
	setupReloads()

	logrus.Info("Starting media repository...")
	if config.Get().PGO.Enabled {
		pgo_internal.Enable(config.Get().PGO.SubmitUrl, config.Get().PGO.SubmitKey)
	}
	metrics.Init()
	web := api.Init()

	// Set up a function to stop everything
	stopAllButWeb := func() {
		logrus.Info("Stopping reload watchers...")
		stopReloads()

		logrus.Info("Stopping metrics...")
		metrics.Stop()

		logrus.Info("Stopping recurring tasks...")
		tasks.StopAll()
	}

	// Set up a listener for SIGINT
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	selfStop := false
	go func() {
		defer close(stop)
		<-stop
		selfStop = true

		logrus.Warn("Stop signal received")
		stopAllButWeb()

		logrus.Info("Stopping web server...")
		api.Stop()
	}()

	// Wait for the web server to exit nicely
	web.Add(1)
	web.Wait()

	// Stop everything else if we have to
	if !selfStop {
		stopAllButWeb()
	}

	// For debugging
	logrus.Info("Goodbye!")
}
