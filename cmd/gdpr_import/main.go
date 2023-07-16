package main

import (
	"errors"
	"flag"
	"os"
	"path"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/archival/v2archive"
	"github.com/turt2live/matrix-media-repo/common/assets"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/logging"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/common/runtime"
	"github.com/turt2live/matrix-media-repo/util/ids"
)

func main() {
	configPath := flag.String("config", "media-repo.yaml", "The path to the configuration")
	migrationsPath := flag.String("migrations", config.DefaultMigrationsPath, "The absolute path for the migrations folder")
	filesDir := flag.String("directory", "./gdpr-data", "The directory for where the entity's exported files are")
	verifyMode := flag.Bool("verify", false, "If set, no media will be imported and instead be tested to see if they've been imported already")
	onlyEntity := flag.String("onlyEntity", "", "The entity (user ID or server name) to import for")
	flag.Parse()

	// Override config path with config for Docker users
	configEnv := os.Getenv("REPO_CONFIG")
	if configEnv != "" {
		configPath = &configEnv
	}

	config.Runtime.IsImportProcess = true // prevents us from creating media by accident
	config.Path = *configPath

	defer assets.Cleanup()
	assets.SetupMigrations(*migrationsPath)

	if ids.GetMachineId() == 0 {
		panic(errors.New("expected custom machine ID for import process (unsafe to import as Machine 0)"))
	}

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

	logrus.Info("Discovering files...")
	fileInfos, err := os.ReadDir(*filesDir)
	if err != nil {
		panic(err)
	}
	files := make([]string, 0)
	for _, f := range fileInfos {
		if f.IsDir() {
			continue
		}
		files = append(files, path.Join(*filesDir, f.Name()))
	}

	// Make an archive reader
	archiver := v2archive.NewReader(rcontext.Initial())

	// Find the manifest
	for _, fname := range files {
		logrus.Debugf("Scanning %s for manifest", fname)
		f, err := os.Open(fname)
		if err != nil {
			panic(err)
		}
		if ok, err := archiver.TryGetManifestFrom(f); err != nil {
			panic(err)
		} else if ok {
			break
		}
	}
	if len(archiver.GetNotUploadedMxcUris()) <= 0 {
		logrus.Warn("Found zero or fewer MXC URIs to import. This usually means there was no manifest found.")
		return
	}
	logrus.Debugf("Importing media for %s", archiver.GetEntityId())

	// Re-process all the files properly this time
	opts := v2archive.ProcessOpts{
		LockedEntityId:    *onlyEntity,
		CheckUploadedOnly: *verifyMode,
	}
	for _, fname := range files {
		logrus.Debugf("Processing %s for media", fname)
		f, err := os.Open(fname)
		if err != nil {
			panic(err)
		}
		if err = archiver.ProcessFile(f, opts); err != nil {
			panic(err)
		}
	}
	if !opts.CheckUploadedOnly {
		if err = archiver.ProcessS3Files(opts); err != nil {
			panic(err)
		}
	}

	missing := archiver.GetNotUploadedMxcUris()
	if len(missing) > 0 {
		for _, mxc := range missing {
			logrus.Warnf("%s has not been uploaded yet - was it included in the package?", mxc)
		}
		logrus.Warnf("%d MXC URIs have not been imported.", len(missing))
	} else if *verifyMode {
		logrus.Info("All MXC URIs have been imported.")
	} else {
		logrus.Info("Import complete.")
	}
}
