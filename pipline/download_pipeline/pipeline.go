package download_pipeline

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/t2bot/go-singleflight-streams"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/pipline/_steps/download"
)

var sf = new(sfstreams.Group)

type DownloadOpts struct {
	FetchRemoteIfNeeded bool
	StartByte           int64
	EndByte             int64
	BlockForReadUntil   time.Duration
}

func (o DownloadOpts) String() string {
	return fmt.Sprintf("f=%t,s=%d,e=%d,b=%s", o.FetchRemoteIfNeeded, o.StartByte, o.EndByte, o.BlockForReadUntil.String())
}

type cancelCloser struct {
	io.ReadCloser
	r      io.ReadCloser
	cancel func()
}

func (c *cancelCloser) Read(p []byte) (int, error) {
	return c.r.Read(p)
}

func (c *cancelCloser) Close() error {
	c.cancel()
	return c.r.Close()
}

func DownloadMedia(ctx rcontext.RequestContext, origin string, mediaId string, opts DownloadOpts) (*database.DbMedia, io.ReadCloser, error) {
	// Step 1: Make our context a timeout context
	var cancel context.CancelFunc
	//goland:noinspection GoVetLostCancel - we handle the function in our custom cancelCloser struct
	ctx.Context, cancel = context.WithTimeout(ctx.Context, opts.BlockForReadUntil)

	// Step 2: Join the singleflight queue
	recordCh := make(chan *database.DbMedia)
	defer close(recordCh)
	r, err, _ := sf.Do(fmt.Sprintf("%s/%s?%s", origin, mediaId, opts.String()), func() (io.ReadCloser, error) {
		serveRecord := func(recordCh chan *database.DbMedia, record *database.DbMedia) {
			recordCh <- record
		}

		// Step 3: Do we already have the media? Serve it if yes.
		mediaDb := database.GetInstance().Media.Prepare(ctx)
		record, err := mediaDb.GetById(origin, mediaId)
		if err != nil {
			return nil, err
		}
		if record != nil {
			go serveRecord(recordCh, record) // async function to prevent deadlock
			return download.OpenStream(ctx, record, opts.StartByte, opts.EndByte)
		}

		// Step 4: Media record unknown - download it (if possible)
		if !opts.FetchRemoteIfNeeded {
			return nil, common.ErrMediaNotFound
		}
		record, r, err := download.TryDownload(ctx, origin, mediaId)
		if err != nil {
			return nil, err
		}
		go serveRecord(recordCh, record) // async function to prevent deadlock

		// Step 5: Limit the stream if needed
		r, err = download.CreateLimitedStream(ctx, r, opts.StartByte, opts.EndByte)
		if err != nil {
			return nil, err
		}

		return r, nil
	})
	if err != nil {
		return nil, nil, err
	}
	record := <-recordCh
	return record, &cancelCloser{
		r:      r,
		cancel: cancel,
	}, nil
}
