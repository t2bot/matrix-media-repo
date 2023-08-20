package pipeline_download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/go-singleflight-streams"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/pipelines/_steps/download"
	"github.com/turt2live/matrix-media-repo/pipelines/_steps/meta"
	"github.com/turt2live/matrix-media-repo/pipelines/_steps/quarantine"
	"github.com/turt2live/matrix-media-repo/util/readers"
)

var sf = new(sfstreams.Group)

type DownloadOpts struct {
	FetchRemoteIfNeeded bool
	StartByte           int64
	EndByte             int64
	BlockForReadUntil   time.Duration
	RecordOnly          bool
}

func (o DownloadOpts) String() string {
	return fmt.Sprintf("f=%t,s=%d,e=%d,b=%s", o.FetchRemoteIfNeeded, o.StartByte, o.EndByte, o.BlockForReadUntil.String())
}

func Execute(ctx rcontext.RequestContext, origin string, mediaId string, opts DownloadOpts) (*database.DbMedia, io.ReadCloser, error) {
	// Step 1: Make our context a timeout context
	var cancel context.CancelFunc
	//goland:noinspection GoVetLostCancel - we handle the function in our custom cancelCloser struct
	ctx.Context, cancel = context.WithTimeout(ctx.Context, opts.BlockForReadUntil)

	// Step 2: Join the singleflight queue
	recordCh := make(chan *database.DbMedia)
	defer close(recordCh)
	r, err, _ := sf.Do(fmt.Sprintf("%s/%s?%s", origin, mediaId, opts.String()), func() (io.ReadCloser, error) {
		serveRecord := func(recordCh chan *database.DbMedia, record *database.DbMedia) {
			defer func() {
				// Don't crash when we send to a closed channel
				recover()
			}()
			recordCh <- record
		}

		// Step 3: Do we already have the media? Serve it if yes.
		mediaDb := database.GetInstance().Media.Prepare(ctx)
		record, err := mediaDb.GetById(origin, mediaId)
		didAsyncWait := false
	handlePossibleRecord:
		if err != nil {
			return nil, err
		}
		if record != nil {
			go serveRecord(recordCh, record) // async function to prevent deadlock
			if record.Quarantined {
				return quarantine.ReturnAppropriateThing(ctx, true, opts.RecordOnly, 512, 512, opts.StartByte, opts.EndByte)
			}
			meta.FlagAccess(ctx, record.Sha256Hash)
			if opts.RecordOnly {
				return nil, nil
			}
			return download.OpenStream(ctx, record.Locatable, opts.StartByte, opts.EndByte)
		}

		// Step 4: Wait for the media, if we can
		if !didAsyncWait {
			record, err = download.WaitForAsyncMedia(ctx, origin, mediaId)
			didAsyncWait = true
			goto handlePossibleRecord
		}

		// Step 5: Media record unknown - download it (if possible)
		if !opts.FetchRemoteIfNeeded {
			return nil, common.ErrMediaNotFound
		}
		record, r, err := download.TryDownload(ctx, origin, mediaId)
		if err != nil {
			return nil, err
		}
		go serveRecord(recordCh, record) // async function to prevent deadlock
		if record.Quarantined {
			return quarantine.ReturnAppropriateThing(ctx, true, opts.RecordOnly, 512, 512, opts.StartByte, opts.EndByte)
		}
		meta.FlagAccess(ctx, record.Sha256Hash)
		if opts.RecordOnly {
			r.Close()
			return nil, nil
		}

		// Step 6: Limit the stream if needed
		r, err = download.CreateLimitedStream(ctx, r, opts.StartByte, opts.EndByte)
		if err != nil {
			return nil, err
		}

		return r, nil
	})
	if errors.Is(err, common.ErrMediaQuarantined) {
		cancel()
		return nil, r, err
	}
	if err != nil {
		cancel()
		return nil, nil, err
	}
	record := <-recordCh
	if opts.RecordOnly {
		if r != nil {
			devErr := errors.New("expected no download stream, but got one anyways")
			ctx.Log.Warn(devErr)
			sentry.CaptureException(devErr)
			r.Close()
		}
		cancel()
		return record, nil, nil
	}
	return record, readers.NewCancelCloser(r, cancel), nil
}
