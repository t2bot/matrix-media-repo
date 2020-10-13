package assets

import (
	"bytes"
	"compress/gzip"
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

func SetupMigrations(givenMigrationsPath string) {
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

	config.Runtime.MigrationsPath = givenMigrationsPath
}

func SetupTemplates(givenTemplatesPath string) {
	if givenTemplatesPath != "" {
		_, err := os.Stat(givenTemplatesPath)
		exists := err == nil || !os.IsNotExist(err)
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

	config.Runtime.TemplatesPath = givenTemplatesPath
}

func SetupAssets(givenAssetsPath string) {
	_, err := os.Stat(givenAssetsPath)
	exists := err == nil || !os.IsNotExist(err)
	if !exists {
		tempAssets, err := ioutil.TempDir(os.TempDir(), "media-repo-assets")
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

		logrus.Infof("Decompressing %s", f)
		gr, err := gzip.NewReader(bytes.NewBuffer(b))
		if err != nil {
			panic(err)
		}
		//noinspection GoDeferInLoop,GoUnhandledErrorResult
		defer gr.Close()
		uncompressedBytes, err := ioutil.ReadAll(gr)
		if err != nil {
			panic(err)
		}

		dest := path.Join(destination, filepath.Base(f))
		logrus.Infof("Writing %s to %s", f, dest)
		err = ioutil.WriteFile(dest, uncompressedBytes, 0644)
		if err != nil {
			panic(err)
		}
	}
}
