package pipeline_thumbnail

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/getsentry/sentry-go"
	sfstreams "github.com/t2bot/go-singleflight-streams"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/pipelines/_steps/download"
	"github.com/turt2live/matrix-media-repo/pipelines/_steps/quarantine"
	"github.com/turt2live/matrix-media-repo/pipelines/_steps/thumbnails"
	"github.com/turt2live/matrix-media-repo/pipelines/pipeline_download"
	"github.com/turt2live/matrix-media-repo/util/readers"
)

var sf = new(sfstreams.Group)

// ThumbnailOpts are options for generating a thumbnail
type ThumbnailOpts struct {
	pipeline_download.DownloadOpts
	Width    int
	Height   int
	Method   string
	Animated bool
}

func (o ThumbnailOpts) String() string {
	return fmt.Sprintf("%s,w=%d,h=%d,m=%s,a=%t", o.DownloadOpts.String(), o.Width, o.Height, o.Method, o.Animated)
}

func (o ThumbnailOpts) ImpliedDownloadOpts() pipeline_download.DownloadOpts {
	return pipeline_download.DownloadOpts{
		FetchRemoteIfNeeded: o.FetchRemoteIfNeeded,
		BlockForReadUntil:   o.BlockForReadUntil,
		RecordOnly:          true,

		// We remove the range parameters to ensure we get a useful download stream
		StartByte: -1,
		EndByte:   -1,
	}
}

func Execute(ctx rcontext.RequestContext, origin string, mediaId string, opts ThumbnailOpts) (*database.DbThumbnail, io.ReadCloser, error) {
	// Step 1: Fix the request parameters
	w, h, method, err1 := thumbnails.PickNewDimensions(ctx, opts.Width, opts.Height, opts.Method)
	if err1 != nil {
		return nil, nil, err1
	}
	opts.Width = w
	opts.Height = h
	opts.Method = method

	// Step 2: Make our context a timeout context
	var cancel context.CancelFunc
	//goland:noinspection GoVetLostCancel - we handle the function in our custom cancelCloser struct
	ctx.Context, cancel = context.WithTimeout(ctx.Context, opts.BlockForReadUntil)

	// Step 3: Join the singleflight queue
	recordCh := make(chan *database.DbThumbnail)
	defer close(recordCh)
	r, err, _ := sf.Do(fmt.Sprintf("%s/%s?%s", origin, mediaId, opts.String()), func() (io.ReadCloser, error) {
		serveRecord := func(recordCh chan *database.DbThumbnail, record *database.DbThumbnail) {
			defer func() {
				// Don't crash when we send to a closed channel
				recover()
			}()
			recordCh <- record
		}

		// Step 4: Get the associated media record (without stream)
		mediaRecord, dr, err := pipeline_download.Execute(ctx, origin, mediaId, opts.ImpliedDownloadOpts())
		if err != nil {
			if err == common.ErrMediaQuarantined {
				go serveRecord(recordCh, nil) // async function to prevent deadlock
				if dr != nil {
					dr.Close()
				}
				return quarantine.ReturnAppropriateThing(ctx, false, opts.RecordOnly, opts.Width, opts.Height, opts.StartByte, opts.EndByte)
			}
			return nil, err
		}
		if mediaRecord == nil {
			return nil, common.ErrMediaNotFound
		}

		// Step 5: See if we're lucky enough to already have this thumbnail
		thumbDb := database.GetInstance().Thumbnails.Prepare(ctx)
		record, err := thumbDb.GetByParams(origin, mediaId, opts.Width, opts.Height, opts.Method, opts.Animated)
		if err != nil {
			return nil, err
		}
		if record != nil {
			go serveRecord(recordCh, record) // async function to prevent deadlock
			if opts.RecordOnly {
				return nil, nil
			}
			return download.OpenStream(ctx, record.Locatable, opts.StartByte, opts.EndByte)
		}

		// Step 6: Generate the thumbnail and return that
		record, r, err := thumbnails.Generate(ctx, mediaRecord, opts.Width, opts.Height, opts.Method, opts.Animated)
		if err != nil {
			return nil, err
		}
		go serveRecord(recordCh, record)
		if opts.RecordOnly {
			defer r.Close()
			return nil, nil
		}

		// Step 7: Create a limited stream
		return download.CreateLimitedStream(ctx, r, opts.StartByte, opts.EndByte)
	})
	if err == common.ErrMediaQuarantined {
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
			devErr := errors.New("expected no thumbnail stream, but got one anyways")
			ctx.Log.Warn(devErr)
			sentry.CaptureException(devErr)
			r.Close()
		}
		cancel()
		return record, nil, nil
	}
	return record, readers.NewCancelCloser(r, cancel), nil
}
