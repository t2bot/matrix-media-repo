package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api/webserver"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/logging"
	"github.com/turt2live/matrix-media-repo/common/version"
	"github.com/turt2live/matrix-media-repo/metrics"
)

func printVersion(usingLogger bool) {
	version.SetDefaults()

	if usingLogger {
		logrus.Info("Version: " + version.Version)
		logrus.Info("Commit: " + version.GitCommit)
	} else {
		fmt.Println("Version: " + version.Version)
		fmt.Println("Commit: " + version.GitCommit)
	}
}

func main() {
	configPath := flag.String("config", "media-repo.yaml", "The path to the configuration")
	migrationsPath := flag.String("migrations", "./migrations", "The absolute path for the migrations folder")
	templatesPath := flag.String("templates", "./templates", "The absolute path for the templates folder")
	versionFlag := flag.Bool("version", false, "Prints the version and exits")
	flag.Parse()

	if *versionFlag {
		printVersion(false)
		return // exit 0
	}

	// Override config path with config for Docker users
	configEnv := os.Getenv("REPO_CONFIG")
	if configEnv != "" {
		configPath = &configEnv
	}

	config.Path = *configPath
	config.Runtime.MigrationsPath = *migrationsPath
	config.Runtime.TemplatesPath = *templatesPath

	err := logging.Setup(config.Get().General.LogDirectory)
	if err != nil {
		panic(err)
	}

	logrus.Info("Starting up...")
	printVersion(true)

	config.PrintDomainInfo()
	loadDatabase()
	loadDatastores()

	logrus.Info("Checking background tasks...")
	err = scanAndStartUnfinishedTasks()
	if err != nil {
		logrus.Fatal(err)
	}

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
	}

	// Set up a listener for SIGINT
	stop := make(chan os.Signal)
	signal.Notify(stop, os.Interrupt, os.Kill)
	selfStop := false
	go func() {
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

	// For debugging
	logrus.Info("Goodbye!")
}
