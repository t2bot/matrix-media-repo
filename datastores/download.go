package datastores

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/metrics"
	"github.com/t2bot/matrix-media-repo/redislib"
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
		if s3c.presignUrl {
			presignedUrl, err := PresignURL(ctx, ds, s3c, dsFileName)
			if err != nil {
				return nil, err
			}
			return nil, redirect(presignedUrl)
		} else {
			return nil, redirect(fmt.Sprintf("%s%s", s3c.publicBaseUrl, dsFileName))
		}
	}

	return Download(ctx, ds, dsFileName)
}

func PresignURL(ctx rcontext.RequestContext, ds config.DatastoreConfig, s3c *s3, dsFileName string) (string, error) {
	url, err := redislib.TryGetURL(ctx, dsFileName)
	if err != nil {
		ctx.Log.Debug("Unable to fetch url from cache due to error: ", err)
	}
	if len(url) == 0 || err != nil {
		presignedUrl, err := s3c.client.PresignedGetObject(ctx.Context, s3c.bucket, dsFileName, time.Duration(s3c.presignExpiry)*time.Second, nil)
		if err != nil {
			return "", err
		}
		presignedUrlStr := presignedUrl.String()
		ctx.Log.Debug("Caching presigned url for: ", dsFileName)
		err = redislib.StoreURL(ctx, dsFileName, presignedUrlStr, time.Duration(s3c.presignCacheExpiry)*time.Second)
		if err != nil {
			ctx.Log.Debug("Not populating url cache due to error: ", err)
		}
		return presignedUrlStr, nil
	} else {
		ctx.Log.Debug("Using cached presigned url for: ", dsFileName)
		return url, nil
	}
}

func WouldRedirectWhenCached(ctx rcontext.RequestContext, ds config.DatastoreConfig) (bool, error) {
	if ds.Type != "s3" {
		return false, nil
	}

	s3c, err := getS3(ds)
	if err != nil {
		return false, err
	}

	return s3c.redirectWhenCached && s3c.publicBaseUrl != "", nil
}
