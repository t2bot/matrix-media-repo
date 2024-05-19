package datastores

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/minio/minio-go/v7"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/metrics"
	"github.com/t2bot/matrix-media-repo/util/ids"
)

func Upload(ctx rcontext.RequestContext, ds config.DatastoreConfig, data io.ReadCloser, size int64, contentType string, sha256hash string) (string, error) {
	defer data.Close()
	hasher := sha256.New()
	tee := io.TeeReader(data, hasher)

	objectName, err := ids.NewUniqueId()
	if err != nil {
		return "", err
	}

	// Suffix the ID so file paths are correctly bucketed
	objectName = fmt.Sprintf("%sidv2fmt", objectName)

	var uploadedBytes int64
	if ds.Type == "s3" {
		var s3c *s3
		s3c, err = getS3(ds)
		if err != nil {
			return "", err
		}

		if s3c.prefixLength > 0 {
			objectName = objectName[:s3c.prefixLength] + "/" + objectName[s3c.prefixLength:]
		}

		metrics.S3Operations.With(prometheus.Labels{"operation": "PutObject"}).Inc()
		var info minio.UploadInfo
		info, err = s3c.client.PutObject(ctx.Context, s3c.bucket, objectName, tee, size, minio.PutObjectOptions{
			StorageClass:     s3c.storageClass,
			ContentType:      contentType,
			DisableMultipart: !s3c.multipartUploads,
		})
		uploadedBytes = info.Size
	} else if ds.Type == "file" {
		basePath := ds.Options["path"]

		var firstContainer string
		var secondContainer string
		var fileName string
		var targetDir string
		var targetFile string

		firstContainer = objectName[0:2]
		secondContainer = objectName[2:4]
		fileName = objectName[4:]
		objectName = path.Join(firstContainer, secondContainer, fileName)
		targetDir = path.Join(basePath, firstContainer, secondContainer)
		targetFile = path.Join(targetDir, fileName)

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
