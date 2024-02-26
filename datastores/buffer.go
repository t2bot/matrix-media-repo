package datastores

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/util/readers"
)

func BufferTemp(datastore config.DatastoreConfig, contents io.ReadCloser) (string, int64, io.ReadCloser, error) {
	fpath := ""
	var err error
	if datastore.Type == "s3" {
		fpath = datastore.Options["tempPath"]
	} else if datastore.Type == "file" {
		fpath, err = os.MkdirTemp(os.TempDir(), "mmr")
		if err != nil {
			return "", 0, nil, fmt.Errorf("error generating temporary directory: %w", err)
		}
	} else {
		return "", 0, nil, errors.New("unknown datastore type - contact developer")
	}

	var target io.Writer
	if fpath == "" {
		logrus.Warnf("Datastore %s does not have a valid temporary path configured. This will lead to increased memory usage.", datastore.Id)
		target = &bytes.Buffer{}
	} else {
		err = os.Mkdir(fpath, os.ModeDir|0o700)
		if err != nil && !os.IsExist(err) {
			return "", 0, nil, fmt.Errorf("error creating temp path: %w", err)
		}
		var file *os.File
		file, err = os.CreateTemp(fpath, "mmr")
		if err != nil {
			return "", 0, nil, fmt.Errorf("error generating temporary file: %w", err)
		}
		target = file
	}

	// Prepare a sha256 calculation
	hasher := sha256.New()

	// Build a multi writer, so we can calculate the hash while we also write to a temporary directory
	mw := io.MultiWriter(hasher, target)

	// Actually copy to the temp file
	var sizeBytes int64
	if sizeBytes, err = io.Copy(mw, contents); err != nil {
		return "", 0, nil, err
	}
	if err = contents.Close(); err != nil {
		return "", 0, nil, err
	}

	// Utility function for finalizing the hash
	hash := func() string {
		return hex.EncodeToString(hasher.Sum(nil))
	}

	// Close out the file and return a read stream (with cleanup function), or return a copy of the byte buffer
	if f, ok := target.(*os.File); ok {
		if err = f.Close(); err != nil {
			return "", 0, nil, err
		}
		f, err = os.Open(f.Name())
		if err != nil {
			return "", 0, nil, err
		}
		return hash(), sizeBytes, readers.NewTempFileCloser(fpath, f.Name(), f), nil
	} else if b, ok := target.(*bytes.Buffer); ok {
		return hash(), sizeBytes, io.NopCloser(b), nil
	} else {
		return "", 0, nil, errors.New("developer error - did not account for possible stream writer type")
	}
}
