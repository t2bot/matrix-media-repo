package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path"

	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/util"
)

func PersistFile(ctx context.Context, file io.Reader, config config.MediaRepoConfig, db Database) (string, error) {
	var basePath string
	var pathSize int64
	for i := 0; i < len(config.Uploads.StoragePaths); i++ {
		currPath := config.Uploads.StoragePaths[i]
		size, err := db.GetSizeOfFolderBytes(ctx, currPath)
		if err != nil {
			continue
		}
		if basePath == "" || size < pathSize {
			basePath = currPath
			pathSize = size
		}
	}

	if basePath == "" {
		return "", errors.New("could not find a suitable base path")
	}

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
			return "", err
		}

		primaryContainer = fileId[0:2]
		secondaryContainer = fileId[2:4]
		fileName = fileId[4:]
		targetDir = path.Join(basePath, primaryContainer, secondaryContainer)
		targetFile = path.Join(targetDir, fileName)

		exists, err = util.FileExists(targetFile)
		attempts++

		// Infinite loop protection
		if attempts > 5 {
			return "", errors.New("failed to find a suitable directory")
		}
	}

	err := os.MkdirAll(targetDir, 0644)
	if err != nil {
		return "", err
	}

	f, err := os.OpenFile(targetFile, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return "", err
	}

	_, err = io.Copy(f, file)
	if err != nil {
		return "", err
	}

	defer f.Close()
	return f.Name(), nil
}

func GetFileHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}

	hasher := sha256.New()

	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	defer f.Close()
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
