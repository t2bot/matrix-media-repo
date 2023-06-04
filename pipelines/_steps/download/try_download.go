package download

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/datastores"
	"github.com/turt2live/matrix-media-repo/errcache"
	"github.com/turt2live/matrix-media-repo/matrix"
	"github.com/turt2live/matrix-media-repo/metrics"
	"github.com/turt2live/matrix-media-repo/pipelines/pipeline_upload"
	"github.com/turt2live/matrix-media-repo/pool"
	"github.com/turt2live/matrix-media-repo/util"
)

type downloadResult struct {
	r           io.ReadCloser
	filename    string
	contentType string
	err         error
}

type uploadResult struct {
	m   *database.DbMedia
	err error
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

		downloadUrl := fmt.Sprintf("%s/_matrix/media/v3/download/%s/%s?allow_remote=false", baseUrl, url.PathEscape(origin), url.PathEscape(mediaId))
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

		if contentLength > 0 && ctx.Config.Downloads.MaxSizeBytes > 0 && contentLength > ctx.Config.Downloads.MaxSizeBytes {
			errFn(common.ErrMediaTooLarge)
			return
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
			r:           resp.Body,
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

	// At this point, res.r is our http response body. We'll first cache it (getting a temporary stream we'll return
	// later), then upload it to persist the record.

	dsConf, err := datastores.Pick(ctx, datastores.RemoteMediaKind)
	if err != nil {
		return nil, nil, err
	}

	pr, pw := io.Pipe()
	tee := io.TeeReader(res.r, pw)
	defer pw.CloseWithError(errors.New("failed to finish write"))
	wg := new(sync.WaitGroup)
	wg.Add(2)
	bufferCh := make(chan downloadResult)
	uploadCh := make(chan uploadResult)
	defer close(bufferCh)
	defer close(uploadCh)

	upstreamClose := func() error { return pw.Close() }

	go func(dsConf config.DatastoreConfig, pr io.ReadCloser, bufferCh chan downloadResult) {
		_, _, retReader, err2 := datastores.BufferTemp(dsConf, pr)
		// async the channel update to avoid deadlocks
		go func(bufferCh chan downloadResult, err2 error, retReader io.ReadCloser) {
			bufferCh <- downloadResult{err: err2, r: retReader}
		}(bufferCh, err2, retReader)
		wg.Done()
	}(dsConf, pr, bufferCh)

	go func(ctx rcontext.RequestContext, origin string, mediaId string, r io.ReadCloser, upstreamClose func() error, contentType string, fileName string, uploadCh chan uploadResult) {
		m, err2 := pipeline_upload.Execute(ctx, origin, mediaId, r, contentType, fileName, "", datastores.RemoteMediaKind)
		// async the channel update to avoid deadlocks
		go func(uploadCh chan uploadResult, err2 error, m *database.DbMedia) {
			uploadCh <- uploadResult{err: err2, m: m}
		}(uploadCh, err2, m)
		if err3 := upstreamClose(); err3 != nil {
			ctx.Log.Warn("Failed to close non-tee writer during remote download: ", err3)
		}
		wg.Done()
	}(ctx, origin, mediaId, io.NopCloser(tee), upstreamClose, res.contentType, res.filename, uploadCh)

	wg.Wait()
	bufferRes := <-bufferCh
	uploadRes := <-uploadCh
	if bufferRes.err != nil {
		return nil, nil, bufferRes.err
	}
	if uploadRes.err != nil {
		defer bufferRes.r.Close()
		return nil, nil, uploadRes.err
	}

	return uploadRes.m, bufferRes.r, nil
}
