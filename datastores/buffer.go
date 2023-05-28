package datastores

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/util/ids"
)

func BufferTemp(datastore config.DatastoreConfig, contents io.ReadCloser) (string, int64, io.ReadCloser, error) {
	fpath := ""
	var err error
	if datastore.Type == "s3" {
		fpath = datastore.Options["tempPath"]
	} else if datastore.Type == "file" {
		var id string
		id, err = ids.NewUniqueId()
		if err != nil {
			return "", 0, nil, errors.New("error generating temporary file ID: " + err.Error())
		}
		fpath = path.Join(os.TempDir(), id)
		fpath, err = os.MkdirTemp(fpath, "mmr")
		if err != nil {
			return "", 0, nil, errors.New("error generating temporary directory: " + err.Error())
		}
	} else {
		return "", 0, nil, errors.New("unknown datastore type - contact developer")
	}

	var target io.Writer
	if fpath == "" {
		logrus.Warnf("Datastore %s does not have a valid temporary path configured. This will lead to increased memory usage.", datastore.Id)
		target = &bytes.Buffer{}
	} else {
		var file *os.File
		file, err = os.CreateTemp(fpath, "mmr")
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
		return hash(), sizeBytes, &tempFileCloser{
			fname:    f.Name(),
			fpath:    fpath,
			upstream: f,
			closed:   false,
		}, nil
	} else if b, ok := target.(*bytes.Buffer); ok {
		return hash(), sizeBytes, io.NopCloser(b), nil
	} else {
		return "", 0, nil, errors.New("developer error - did not account for possible stream writer type")
	}
}

type tempFileCloser struct {
	io.ReadCloser
	fname    string
	fpath    string
	upstream io.ReadCloser
	closed   bool
}

func (c *tempFileCloser) Close() error {
	if c.closed {
		return nil
	}
	var err error
	if err = os.Remove(c.fname); err != nil {
		return err
	}
	if err = os.Remove(c.fpath); err != nil {
		return err
	}
	c.closed = true
	return c.upstream.Close()
}

func (c *tempFileCloser) Read(p []byte) (n int, err error) {
	return c.upstream.Read(p)
}
