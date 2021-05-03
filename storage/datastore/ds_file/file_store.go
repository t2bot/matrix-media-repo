package ds_file

import (
	"errors"
	"io"
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
		fileId, err := util.GenerateRandomString(64)
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

	err = PersistFileAtLocation(targetFile, file, ctx)
	if err != nil {
		return nil, err
	}

	locationPath := path.Join(primaryContainer, secondaryContainer, fileName)
	return &types.ObjectInfo{
		Location: locationPath,
	}, nil
}

func PersistFileAtLocation(targetFile string, file io.ReadCloser, ctx rcontext.RequestContext) error {
	defer cleanup.DumpAndCloseStream(file)

	f, err := os.OpenFile(targetFile, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer cleanup.DumpAndCloseStream(f)

	var sizeBytes int64
	var writeErr error

	ctx.Log.Info("Writing file...")
	sizeBytes, writeErr = io.Copy(f, file)
	if writeErr != nil {
		return writeErr
	}
	ctx.Log.Info("Wrote ", sizeBytes, " bytes to file")

	return nil
}

func DeletePersistedFile(basePath string, location string) error {
	return os.Remove(path.Join(basePath, location))
}
