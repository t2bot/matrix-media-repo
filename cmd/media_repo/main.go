package main

import (
	"flag"
	"os"
	"os/signal"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api/webserver"
	"github.com/turt2live/matrix-media-repo/common/assets"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/logging"
	"github.com/turt2live/matrix-media-repo/common/runtime"
	"github.com/turt2live/matrix-media-repo/common/version"
	"github.com/turt2live/matrix-media-repo/internal_cache"
	"github.com/turt2live/matrix-media-repo/metrics"
	"github.com/turt2live/matrix-media-repo/tasks"
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
	assets.SetupMigrations(*migrationsPath)
	assets.SetupTemplates(*templatesPath)
	assets.SetupAssets(*assetsPath)

	err := logging.Setup(config.Get().General.LogDirectory)
	if err != nil {
		panic(err)
	}

	logrus.Info("Starting up...")
	runtime.RunStartupSequence()
	internal_cache.ReplaceInstance() // init the cache as we may be using Redis, and it'd be good to get going sooner

	logrus.Info("Checking background tasks...")
	err = scanAndStartUnfinishedTasks()
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Info("Starting recurring tasks...")
	tasks.StartAll()

	logrus.Info("Starting config watcher...")
	watcher := config.Watch()
	defer watcher.Close()
	setupReloads()

	logrus.Info("Starting media repository...")
	metrics.Init()
	web := webserver.Init()

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
	stop := make(chan os.Signal)
	signal.Notify(stop, os.Interrupt, os.Kill)
	selfStop := false
	go func() {
		defer close(stop)
		<-stop
		selfStop = true

		logrus.Warn("Stop signal received")
		stopAllButWeb()

		logrus.Info("Stopping web server...")
		webserver.Stop()
	}()

	// Wait for the web server to exit nicely
	web.Add(1)
	web.Wait()

	// Stop everything else if we have to
	if !selfStop {
		stopAllButWeb()
	}

	// Clean up
	assets.Cleanup()

	// For debugging
	logrus.Info("Goodbye!")
}
