package main

import (
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/assets"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/logging"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/common/runtime"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"os"
)

func main() {
	configPath := flag.String("config", "media-repo.yaml", "The path to the configuration")
	datastoreId := flag.String("datastoreId", "", "The datastore ID to check (must be an S3 datastore)")
	outFile := flag.String("outFile", "./s3-probably-safe-to-delete.txt", "File path for where to write results")
	migrationsPath := flag.String("migrations", config.DefaultMigrationsPath, "The absolute path for the migrations folder")
	templatesPath := flag.String("templates", config.DefaultTemplatesPath, "The absolute path for the templates folder")
	flag.Parse()

	// Override config path with config for Docker users
	configEnv := os.Getenv("REPO_CONFIG")
	if configEnv != "" {
		configPath = &configEnv
	}

	config.Path = *configPath
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

	logrus.Info("Scanning datastore: ", *datastoreId)
	ctx := rcontext.Initial().LogWithFields(logrus.Fields{"datastoreId": *datastoreId})
	ds, err := datastore.LocateDatastore(ctx, *datastoreId)
	if err != nil {
		panic(err)
	}

	objectIds, err := ds.ListObjectIds(ctx)
	if err != nil {
		panic(err)
	}
	logrus.Infof("Got %d object IDs", len(objectIds))

	mediaDb := storage.GetDatabase().GetMediaStore(ctx)
	thumbsDb := storage.GetDatabase().GetThumbnailStore(ctx)
	usedLocations := make([]string, 0)

	logrus.Info("Scanning media for datastore: ", *datastoreId)
	locations, err := mediaDb.GetDistinctLocationsForDatastore(*datastoreId)
	if err != nil {
		panic(err)
	}

	for _, l := range locations {
		usedLocations = append(usedLocations, l)
	}

	logrus.Infof("Got %d locations", len(locations))

	logrus.Info("Scanning thumbnails for datastore: ", *datastoreId)
	locations, err = thumbsDb.GetDistinctLocationsForDatastore(*datastoreId)
	if err != nil {
		panic(err)
	}

	for _, l := range locations {
		usedLocations = append(usedLocations, l)
	}

	logrus.Infof("Got %d locations", len(locations))

	logrus.Info("Comparing locations known in DB to S3...")
	probablyAbleToDelete := make([]string, 0)
	for _, s3Location := range objectIds {
		keep := false
		for _, dbLocation := range usedLocations {
			if dbLocation == s3Location {
				keep = true
				break
			}
		}
		if keep {
			continue
		}
		logrus.Warnf("%s might be safe to delete from s3", s3Location)
		probablyAbleToDelete = append(probablyAbleToDelete, s3Location)
	}

	logrus.Info("Writing file for probably-safe-to-delete object IDs...")
	f, err := os.Create(*outFile)
	defer f.Close()
	if err != nil {
		panic(err)
	}
	outString := ""
	for _, id := range probablyAbleToDelete {
		outString += fmt.Sprintf("%s\n", id)
	}
	_, err = f.WriteString(outString)
	if err != nil {
		panic(err)
	}
	logrus.Info("Done!")
}
