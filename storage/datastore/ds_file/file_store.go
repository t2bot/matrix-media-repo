package ds_file

import (
	"errors"
	"github.com/turt2live/matrix-media-repo/util/ids"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
)

func PersistFile(basePath string, file io.ReadCloser, ctx rcontext.RequestContext) (*types.ObjectInfo, error) {
	defer cleanup.DumpAndCloseStream(file)

	exists := true
	var primaryContainer string
	var secondaryContainer string
	var fileName string
	var targetDir string
	var targetFile string
	attempts := 0
	for exists {
		fileId, err := ids.NewUniqueId()
		if err != nil {
			return nil, err
		}

		primaryContainer = fileId[0:2]
		secondaryContainer = fileId[2:4]
		fileName = fileId[4:]
		targetDir = path.Join(basePath, primaryContainer, secondaryContainer)
		targetFile = path.Join(targetDir, fileName)

		ctx.Log.Info("Checking if file exists: " + targetFile)

		exists, err = util.FileExists(targetFile)
		attempts++

		if err != nil {
			ctx.Log.Error("Error checking if the file exists: " + err.Error())
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

	sizeBytes, hash, err := PersistFileAtLocation(targetFile, file, ctx)
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

func PersistFileAtLocation(targetFile string, file io.ReadCloser, ctx rcontext.RequestContext) (int64, string, error) {
	defer cleanup.DumpAndCloseStream(file)

	f, err := os.OpenFile(targetFile, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return 0, "", err
	}
	defer cleanup.DumpAndCloseStream(f)

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
		ctx.Log.Info("Calculating hash of stream...")
		hash, hashErr = util.GetSha256HashOfStream(ioutil.NopCloser(tr))
		ctx.Log.Info("Hash of file is ", hash)
		done <- true
	}()

	go func() {
		ctx.Log.Info("Writing file...")
		sizeBytes, writeErr = io.Copy(f, rfile)
		ctx.Log.Info("Wrote ", sizeBytes, " bytes to file")
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
	err := os.Remove(path.Join(basePath, location))
	if err != nil && os.IsNotExist(err) {
		// It didn't exist, so pretend it was successful
		return nil
	}
	return err
}
