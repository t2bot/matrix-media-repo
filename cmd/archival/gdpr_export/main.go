package main

import (
	"flag"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/archival"
	"github.com/turt2live/matrix-media-repo/common/assets"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/logging"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/common/runtime"
)

func main() {
	configPath := flag.String("config", "media-repo.yaml", "The path to the configuration")
	migrationsPath := flag.String("migrations", config.DefaultMigrationsPath, "The absolute path for the migrations folder")
	templatesPath := flag.String("templates", config.DefaultTemplatesPath, "The absolute path for the templates folder")
	entity := flag.String("entity", "", "The user ID or server name to export")
	destination := flag.String("destination", "./gdpr-data", "The directory for where export files should be placed")
	flag.Parse()

	// Override config path with config for Docker users
	configEnv := os.Getenv("REPO_CONFIG")
	if configEnv != "" {
		configPath = &configEnv
	}

	if *entity == "" {
		flag.Usage()
		os.Exit(1)
		return
	}

	config.Runtime.IsImportProcess = true // prevents us from creating media by accident
	config.Path = *configPath

	defer assets.Cleanup()
	assets.SetupMigrations(*migrationsPath)
	assets.SetupTemplates(*templatesPath)

	var err error
	err = logging.Setup(
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

	logrus.Info("Starting export...")
	ctx := rcontext.Initial()
	err = archival.ExportEntityData(ctx, "OOB", *entity, true, archival.PersistPartsToDirectory(*destination))
	if err != nil {
		panic(err)
	}

	logrus.Infof("Export complete! Files for %s should be in %s", *entity, *destination)
}
