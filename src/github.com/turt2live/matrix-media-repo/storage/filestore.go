package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path"
)

func PersistTempFile(filePath string) (string, error) {
	// TODO: Load balance stores
	// TODO: Use config for stores
	basePath := os.TempDir()
	hash, err := GetFileHash(filePath)
	if err != nil {
		return "", err
	}

	primaryContainer := hash[0:2]
	secondaryContainer := hash[2:4]
	fileName := hash[4:]

	targetDir := path.Join(basePath, primaryContainer, secondaryContainer)
	err = os.MkdirAll(targetDir, 0644)
	if err != nil {
		return "", err
	}

	targetFile := path.Join(targetDir, fileName)
	err = os.Rename(filePath, targetFile)
	if err != nil {
		return "", err
	}

	return targetFile, nil
}

func GetFileHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}

	hasher := sha256.New()

	defer f.Close()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}