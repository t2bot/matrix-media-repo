package assets

import (
	"encoding/hex"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
)

var tempMigrations string
var tempTemplates string

func SetupTemplatesAndMigrations(givenMigrationsPath string, givenTemplatesPath string) {
	_, err := os.Stat(givenMigrationsPath)
	exists := err == nil || !os.IsNotExist(err)
	if !exists {
		tempMigrations, err = ioutil.TempDir(os.TempDir(), "media-repo-migrations")
		if err != nil {
			panic(err)
		}
		logrus.Info("Migrations path doesn't exist - attempting to unpack from compiled data")
		extractPrefixTo("migrations", tempMigrations)
		givenMigrationsPath = tempMigrations
	}

	if givenTemplatesPath != "" {
		_, err = os.Stat(givenTemplatesPath)
		exists = err == nil || !os.IsNotExist(err)
		if !exists {
			tempTemplates, err = ioutil.TempDir(os.TempDir(), "media-repo-templates")
			if err != nil {
				panic(err)
			}
			logrus.Info("Templates path doesn't exist - attempting to unpack from compiled data")
			extractPrefixTo("templates", tempTemplates)
			givenTemplatesPath = tempTemplates
		}
	}

	config.Runtime.MigrationsPath = givenMigrationsPath
	config.Runtime.TemplatesPath = givenTemplatesPath
}

func Cleanup() {
	if tempMigrations != "" {
		logrus.Info("Cleaning up temporary assets directory: ", tempMigrations)
		os.Remove(tempMigrations)
	}
	if tempTemplates != "" {
		logrus.Info("Cleaning up temporary assets directory: ", tempTemplates)
		os.Remove(tempTemplates)
	}
}

func extractPrefixTo(pathName string, destination string) {
	for f, h := range compressedFiles {
		if !strings.HasPrefix(f, pathName) {
			continue
		}

		logrus.Infof("Decoding %s", f)
		b, err := hex.DecodeString(h)
		if err != nil {
			panic(err)
		}

		dest := path.Join(destination, filepath.Base(f))
		logrus.Infof("Writing %s to %s", f, dest)
		err = ioutil.WriteFile(dest, b, 0644)
		if err != nil {
			panic(err)
		}
	}
}
