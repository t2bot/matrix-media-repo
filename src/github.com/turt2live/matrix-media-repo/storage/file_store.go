package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/util"
)

func PersistFile(file io.Reader, ctx context.Context, log *logrus.Entry) (string, error) {
	var basePath string
	var pathSize int64
	for i := 0; i < len(config.Get().Uploads.StoragePaths); i++ {
		currPath := config.Get().Uploads.StoragePaths[i]
		size, err := GetDatabase().GetSizeOfFolderBytes(ctx, currPath)
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
			return "", err
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
			return "", errors.New("failed to find a suitable directory")
		}
	}

	err := os.MkdirAll(targetDir, 0755)
	if err != nil {
		return "", err
	}

	f, err := os.OpenFile(targetFile, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return "", err
	}
	defer f.Close()

	_, err = io.Copy(f, file)
	if err != nil {
		return "", err
	}

	return f.Name(), nil
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

func GetFileContentType(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}

	buffer := make([]byte, 512)

	_, err = f.Read(buffer)
	defer f.Close()
	if err != nil {
		return "", err
	}

	return http.DetectContentType(buffer), nil
}
