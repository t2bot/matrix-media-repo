package datastores

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"

	"github.com/minio/minio-go/v7"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/metrics"
	"github.com/turt2live/matrix-media-repo/util/ids"
)

func Upload(ctx rcontext.RequestContext, ds config.DatastoreConfig, data io.ReadCloser, size int64, contentType string, sha256hash string) (string, error) {
	defer data.Close()
	hasher := sha256.New()
	tee := io.TeeReader(data, hasher)
	var objectName string
	var err error

	var uploadedBytes int64
	if ds.Type == "s3" {
		var s3c *s3
		s3c, err = getS3(ds)
		if err != nil {
			return "", err
		}

		// Ensure unique ID
		exists := true
		attempts := 0
		for exists {
			objectName, err = ids.NewUniqueId()
			if err != nil {
				return "", err
			}

			attempts++
			if attempts > 10 {
				return "", errors.New("failed to generate suitable object name for S3 store")
			}
			metrics.S3Operations.With(prometheus.Labels{"operation": "StatObject"}).Inc()
			_, err = s3c.client.StatObject(ctx.Context, s3c.bucket, objectName, minio.StatObjectOptions{})
			if err != nil {
				var merr minio.ErrorResponse
				if errors.As(err, &merr) {
					if merr.Code == "NoSuchKey" || merr.StatusCode == http.StatusNotFound {
						exists = false
					}
				}
			}
		}

		metrics.S3Operations.With(prometheus.Labels{"operation": "PutObject"}).Inc()
		var info minio.UploadInfo
		info, err = s3c.client.PutObject(ctx.Context, s3c.bucket, objectName, tee, size, minio.PutObjectOptions{StorageClass: s3c.storageClass, ContentType: contentType})
		uploadedBytes = info.Size
	} else if ds.Type == "file" {
		basePath := ds.Options["path"]

		var firstContainer string
		var secondContainer string
		var fileName string
		var targetDir string
		var targetFile string

		// Ensure unique ID
		exists := true
		attempts := 0
		for exists {
			objectName, err = ids.NewUniqueId()
			if err != nil {
				return "", err
			}

			attempts++
			if attempts > 10 {
				return "", errors.New("failed to generate suitable file name for persistence")
			}

			firstContainer = objectName[0:2]
			secondContainer = objectName[2:4]
			fileName = objectName[4:]
			objectName = path.Join(firstContainer, secondContainer, fileName)
			targetDir = path.Join(basePath, firstContainer, secondContainer)
			targetFile = path.Join(targetDir, fileName)

			_, err = os.Stat(targetFile)
			if err != nil && !os.IsNotExist(err) {
				return "", err
			} else if err != nil && os.IsNotExist(err) {
				exists = false
			}
		}

		// Persist file
		var file *os.File
		if err = os.MkdirAll(targetDir, 0755); err != nil {
			return "", err
		}
		file, err = os.OpenFile(targetFile, os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			return "", err
		}
		uploadedBytes, err = io.Copy(file, tee)
		if err != nil {
			return "", err
		}
		err = file.Close()
	} else {
		return "", errors.New("unknown datastore type - contact developer")
	}

	if err != nil {
		return "", err
	}
	if uploadedBytes != size {
		if err = Remove(ctx, ds, objectName); err != nil {
			ctx.Log.Warn("Error deleting upload (delete attempted due to persistence error): ", err)
		}
		return "", fmt.Errorf("upload size mismatch: expected %d got %d bytes", size, uploadedBytes)
	}

	uploadedHash := hex.EncodeToString(hasher.Sum(nil))
	if uploadedHash != sha256hash {
		if err = Remove(ctx, ds, objectName); err != nil {
			ctx.Log.Warn("Error deleting upload (delete attempted due to persistence error): ", err)
		}
		return "", fmt.Errorf("upload hash mismatch: expected %s got %s", sha256hash, uploadedHash)
	}

	return objectName, nil
}
