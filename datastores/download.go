package datastores

import (
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
)

func Download(ctx rcontext.RequestContext, ds config.DatastoreConfig, dsFileName string) (io.ReadSeekCloser, error) {
	var err error
	var rsc io.ReadSeekCloser
	if ds.Type == "s3" {
		var s3c *s3
		s3c, err = getS3(ds)
		if err != nil {
			return nil, err
		}

		metrics.S3Operations.With(prometheus.Labels{"operation": "GetObject"}).Inc()
		rsc, err = s3c.client.GetObject(ctx.Context, s3c.bucket, dsFileName, minio.GetObjectOptions{})
	} else if ds.Type == "file" {
		basePath := ds.Options["path"]

		rsc, err = os.Open(path.Join(basePath, dsFileName))
	} else {
		return nil, errors.New("unknown datastore type - contact developer")
	}

	return rsc, err
}

func DownloadOrRedirect(ctx rcontext.RequestContext, ds config.DatastoreConfig, dsFileName string) (io.ReadSeekCloser, error) {
	if ds.Type != "s3" {
		return Download(ctx, ds, dsFileName)
	}

	s3c, err := getS3(ds)
	if err != nil {
		return nil, err
	}

	if s3c.publicBaseUrl != "" {
		metrics.S3Operations.With(prometheus.Labels{"operation": "RedirectGetObject"}).Inc()
		return nil, redirect(fmt.Sprintf("%s%s", s3c.publicBaseUrl, dsFileName))
	}

	return Download(ctx, ds, dsFileName)
}
