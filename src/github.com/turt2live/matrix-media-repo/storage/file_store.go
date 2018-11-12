package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

func PersistFile(file io.Reader, ctx context.Context, log *logrus.Entry) (*types.Datastore, string, error) {
	var basePath string
	if len(config.Get().Uploads.StoragePaths) != 1 {
		var pathSize int64
		for i := 0; i < len(config.Get().Uploads.StoragePaths); i++ {
			currPath := config.Get().Uploads.StoragePaths[i]
			size, err := GetDatabase().GetMetadataStore(ctx, log).GetSizeOfFolderBytes(currPath)
			if err != nil {
				continue
			}
			if basePath == "" || size < pathSize {
				basePath = currPath
				pathSize = size
			}
		}
	} else {
		basePath = config.Get().Uploads.StoragePaths[0]
	}

	if basePath == "" {
		return nil, "", errors.New("could not find a suitable base path")
	}
	log.Info("Using the base path: " + basePath)

	exists := true
	var primaryContainer string
	var secondaryContainer string
	var fileName string
	var targetDir string
	var targetFile string
	attempts := 0
	for exists {
		fileId, err := util.GenerateRandomString(64)
		if err != nil {
			return nil, "", err
		}

		primaryContainer = fileId[0:2]
		secondaryContainer = fileId[2:4]
		fileName = fileId[4:]
		targetDir = path.Join(basePath, primaryContainer, secondaryContainer)
		targetFile = path.Join(targetDir, fileName)

		log.Info("Checking if file exists: " + targetFile)

		exists, err = util.FileExists(targetFile)
		attempts++

		if err != nil {
			log.Error("Error checking if the file exists: " + err.Error())
		}

		// Infinite loop protection
		if attempts > 5 {
			return nil, "", errors.New("failed to find a suitable directory")
		}
	}

	err := os.MkdirAll(targetDir, 0755)
	if err != nil {
		return nil, "", err
	}

	f, err := os.OpenFile(targetFile, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	_, err = io.Copy(f, file)
	if err != nil {
		return nil, "", err
	}

	locationPath := path.Join(primaryContainer, secondaryContainer, fileName)
	datastorePath := basePath

	datastore, err := GetOrCreateDatastore(ctx, log, datastorePath)
	if err != nil {
		return nil, "", err
	}

	return datastore, locationPath, nil
}

func GetFileHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()

	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
