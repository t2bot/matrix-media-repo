package ds_file

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

func PersistFile(basePath string, file io.ReadCloser, ctx context.Context, log *logrus.Entry) (*types.ObjectInfo, error) {
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
			return nil, err
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
			return nil, errors.New("failed to find a suitable directory")
		}
	}

	err := os.MkdirAll(targetDir, 0755)
	if err != nil {
		return nil, err
	}

	sizeBytes, hash, err := PersistFileAtLocation(targetFile, file, ctx, log)
	if err != nil {
		return nil, err
	}

	locationPath := path.Join(primaryContainer, secondaryContainer, fileName)
	return &types.ObjectInfo{
		Location:   locationPath,
		Sha256Hash: hash,
		SizeBytes:  sizeBytes,
	}, nil
}

func PersistFileAtLocation(targetFile string, file io.ReadCloser, ctx context.Context, log *logrus.Entry) (int64, string, error) {
	f, err := os.OpenFile(targetFile, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()

	rfile, wfile := io.Pipe()
	tr := io.TeeReader(file, wfile)

	done := make(chan bool)
	defer close(done)

	var hash string
	var sizeBytes int64
	var hashErr error
	var writeErr error

	go func() {
		defer wfile.Close()
		log.Info("Calculating hash of stream...")
		hash, hashErr = util.GetSha256HashOfStream(ioutil.NopCloser(tr))
		log.Info("Hash of file is ", hash)
		done <- true
	}()

	go func() {
		log.Info("Writing file...")
		sizeBytes, writeErr = io.Copy(f, rfile)
		log.Info("Wrote ", sizeBytes, " bytes to file")
		done <- true
	}()

	for c := 0; c < 2; c++ {
		<-done
	}

	if hashErr != nil {
		defer os.Remove(targetFile)
		return 0, "", hashErr
	}

	if writeErr != nil {
		return 0, "", writeErr
	}

	return sizeBytes, hash, nil
}

func DeletePersistedFile(basePath string, location string) error {
	return os.Remove(path.Join(basePath, location))
}
