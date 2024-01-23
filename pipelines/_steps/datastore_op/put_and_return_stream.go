package datastore_op

import (
	"errors"
	"io"
	"sync"

	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/datastores"
	"github.com/t2bot/matrix-media-repo/pipelines/pipeline_upload"
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

func PutAndReturnStream(ctx rcontext.RequestContext, origin string, mediaId string, input io.ReadCloser, contentType string, fileName string, kind datastores.Kind) (*database.DbMedia, io.ReadCloser, error) {
	dsConf, err := datastores.Pick(ctx, kind)
	if err != nil {
		return nil, nil, err
	}

	pr, pw := io.Pipe()
	tee := io.TeeReader(input, pw)
	defer func(pw *io.PipeWriter, err error) {
		_ = pw.CloseWithError(err)
	}(pw, errors.New("failed to finish write"))

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

	go func(ctx rcontext.RequestContext, origin string, mediaId string, r io.ReadCloser, upstreamClose func() error, contentType string, fileName string, kind datastores.Kind, uploadCh chan uploadResult) {
		m, err2 := pipeline_upload.Execute(ctx, origin, mediaId, r, contentType, fileName, "", kind)
		// async the channel update to avoid deadlocks
		go func(uploadCh chan uploadResult, err2 error, m *database.DbMedia) {
			uploadCh <- uploadResult{err: err2, m: m}
		}(uploadCh, err2, m)
		if err3 := upstreamClose(); err3 != nil {
			ctx.Log.Warn("Failed to close non-tee writer during remote download: ", err3)
		}
		wg.Done()
	}(ctx, origin, mediaId, io.NopCloser(tee), upstreamClose, contentType, fileName, kind, uploadCh)

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
