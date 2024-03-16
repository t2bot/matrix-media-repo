package download

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/datastores"
	"github.com/t2bot/matrix-media-repo/errcache"
	"github.com/t2bot/matrix-media-repo/matrix"
	"github.com/t2bot/matrix-media-repo/metrics"
	"github.com/t2bot/matrix-media-repo/pipelines/steps/datastore_op"
	"github.com/t2bot/matrix-media-repo/pool"
	"github.com/t2bot/matrix-media-repo/util"
	"github.com/t2bot/matrix-media-repo/util/readers"
)

type downloadResult struct {
	r           io.ReadCloser
	filename    string
	contentType string
	err         error
}

func TryDownload(ctx rcontext.RequestContext, origin string, mediaId string) (*database.DbMedia, io.ReadCloser, error) {
	if util.IsServerOurs(origin) {
		return nil, nil, common.ErrMediaNotFound
	}

	ch := make(chan downloadResult)
	defer close(ch)
	fn := func() {
		cacheKey := fmt.Sprintf("%s/%s", origin, mediaId)
		if err := errcache.DownloadErrors.Get(cacheKey); err != nil {
			ch <- downloadResult{err: err}
			return
		}

		errFn := func(err error) {
			errcache.DownloadErrors.Set(cacheKey, err)
			ch <- downloadResult{err: err}
		}

		baseUrl, realHost, err := matrix.GetServerApiUrl(origin)
		if err != nil {
			errFn(err)
			return
		}

		downloadUrl := fmt.Sprintf("%s/_matrix/media/v3/download/%s/%s?allow_remote=false&allow_redirect=true", baseUrl, url.PathEscape(origin), url.PathEscape(mediaId))
		resp, err := matrix.FederatedGet(downloadUrl, realHost, ctx)
		metrics.MediaDownloaded.With(prometheus.Labels{"origin": origin}).Inc()
		if err != nil {
			errFn(err)
			return
		}

		if resp.StatusCode == http.StatusNotFound {
			errFn(common.ErrMediaNotFound)
			return
		} else if resp.StatusCode != http.StatusOK {
			errFn(errors.New(fmt.Sprintf("unexpected status code %d", resp.StatusCode)))
			return
		}

		contentLength := int64(0)
		if resp.Header.Get("Content-Length") != "" {
			contentLength, err = strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
			if err != nil {
				errFn(err)
				return
			}
		}

		if contentLength != 0 && ctx.Config.Downloads.MaxSizeBytes > 0 && contentLength > ctx.Config.Downloads.MaxSizeBytes {
			errFn(common.ErrMediaTooLarge)
			return
		}

		r := resp.Body
		if ctx.Config.Downloads.MaxSizeBytes > 0 {
			r = readers.LimitReaderWithOverrunError(resp.Body, ctx.Config.Downloads.MaxSizeBytes)
		}

		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream" // binary
		}

		fileName := "download"
		_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition"))
		if err == nil && params["filename"] != "" {
			fileName = params["filename"]
		}

		ch <- downloadResult{
			r:           r,
			filename:    fileName,
			contentType: contentType,
			err:         nil,
		}
	}
	if err := pool.DownloadQueue.Schedule(fn); err != nil {
		return nil, nil, err
	}
	res := <-ch
	if res.err != nil {
		return nil, nil, res.err
	}

	// At this point, res.r is our http response body.

	return datastore_op.PutAndReturnStream(ctx, origin, mediaId, res.r, res.contentType, res.filename, datastores.RemoteMediaKind)
}
