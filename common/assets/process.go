package assets

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common/config"
)

var tempMigrations string
var tempTemplates string
var tempAssets string

func SetupMigrations(givenMigrationsPath string) {
	_, err := os.Stat(givenMigrationsPath)
	exists := err == nil || !os.IsNotExist(err)
	if !exists {
		tempMigrations, err = os.MkdirTemp(os.TempDir(), "media-repo-migrations")
		if err != nil {
			panic(err)
		}
		logrus.Info("Migrations path doesn't exist - attempting to unpack from compiled data")
		extractPrefixTo("migrations", tempMigrations)
		givenMigrationsPath = tempMigrations
	}

	config.Runtime.MigrationsPath = givenMigrationsPath
}

func SetupTemplates(givenTemplatesPath string) {
	if givenTemplatesPath != "" {
		_, err := os.Stat(givenTemplatesPath)
		exists := err == nil || !os.IsNotExist(err)
		if !exists {
			tempTemplates, err = os.MkdirTemp(os.TempDir(), "media-repo-templates")
			if err != nil {
				panic(err)
			}
			logrus.Info("Templates path doesn't exist - attempting to unpack from compiled data")
			extractPrefixTo("templates", tempTemplates)
			givenTemplatesPath = tempTemplates
		}
	}

	config.Runtime.TemplatesPath = givenTemplatesPath
}

func SetupAssets(givenAssetsPath string) {
	_, err := os.Stat(givenAssetsPath)
	exists := err == nil || !os.IsNotExist(err)
	if !exists {
		tempAssets, err = os.MkdirTemp(os.TempDir(), "media-repo-assets")
		if err != nil {
			panic(err)
		}
		logrus.Info("Assets path doesn't exist - attempting to unpack from compiled data")
		extractPrefixTo("assets", tempAssets)
		givenAssetsPath = tempAssets
	}

	config.Runtime.AssetsPath = givenAssetsPath
}

func Cleanup() {
	if tempMigrations != "" {
		logrus.Info("Cleaning up temporary migrations directory: ", tempMigrations)
		os.Remove(tempMigrations)
	}
	if tempTemplates != "" {
		logrus.Info("Cleaning up temporary assets directory: ", tempTemplates)
		os.Remove(tempTemplates)
	}
	if tempAssets != "" {
		logrus.Info("Cleaning up temporary assets directory: ", tempAssets)
		os.Remove(tempAssets)
	}
}

func extractPrefixTo(pathName string, destination string) {
	for f, b64 := range f2CompressedFiles {
		if !strings.HasPrefix(f, pathName) {
			continue
		}

		b, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			panic(err)
		}

		gr, err := gzip.NewReader(bytes.NewBuffer(b))
		if err != nil {
			panic(err)
		}

		dest := path.Join(destination, filepath.Base(f))
		logrus.Debugf("Writing %s to %s", f, dest)
		file, err := os.Create(dest)
		if err != nil {
			panic(err)
		}

		_, err = io.Copy(file, gr)
		if err != nil {
			panic(err)
		}

		_ = gr.Close()
		file.Close()
	}
}
