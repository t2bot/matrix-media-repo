package main

import (
	"flag"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api/webserver"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/logging"
)

func main() {
	configPath := flag.String("config", "media-repo.yaml", "The path to the configuration")
	migrationsPath := flag.String("migrations", "./migrations", "The absolute path the migrations folder")
	flag.Parse()

	config.Path = *configPath
	config.Runtime.MigrationsPath = *migrationsPath

	err := logging.Setup(config.Get().General.LogDirectory)
	if err != nil {
		panic(err)
	}

	logrus.Info("Starting media repository...")
	webserver.Init() // blocks to listen for requests
}
