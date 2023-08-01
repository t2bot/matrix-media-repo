package datastores

import (
	"errors"
	"os"
	"path"

	"github.com/minio/minio-go/v7"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/metrics"
)

func Remove(ctx rcontext.RequestContext, ds config.DatastoreConfig, location string) error {
	var err error
	if ds.Type == "s3" {
		var s3c *s3
		s3c, err = getS3(ds)
		if err != nil {
			return err
		}

		metrics.S3Operations.With(prometheus.Labels{"operation": "RemoveObject"}).Inc()
		err = s3c.client.RemoveObject(ctx.Context, s3c.bucket, location, minio.RemoveObjectOptions{})
	} else if ds.Type == "file" {
		basePath := ds.Options["path"]
		err = os.Remove(path.Join(basePath, location))
		if err != nil && os.IsNotExist(err) {
			return nil // not existing means it was deleted, as far as we care
		}
	} else {
		return errors.New("unknown datastore type - contact developer")
	}

	return err
}

func RemoveWithDsId(ctx rcontext.RequestContext, dsId string, location string) error {
	ds, ok := Get(ctx, dsId)
	if !ok {
		return errors.New("unknown datastore")
	}
	return Remove(ctx, ds, location)
}
