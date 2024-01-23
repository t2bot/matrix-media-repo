package pipeline_download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/go-singleflight-streams"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/pipelines/_steps/download"
	"github.com/t2bot/matrix-media-repo/pipelines/_steps/meta"
	"github.com/t2bot/matrix-media-repo/pipelines/_steps/quarantine"
	"github.com/t2bot/matrix-media-repo/util/readers"
	"github.com/t2bot/matrix-media-repo/util/sfcache"
)

var streamSf = new(sfstreams.Group)
var recordSf = sfcache.NewSingleflightCache[*database.DbMedia]()

func init() {
	streamSf.UseSeekers = true
}

type DownloadOpts struct {
	FetchRemoteIfNeeded bool
	BlockForReadUntil   time.Duration
	RecordOnly          bool
	CanRedirect         bool
}

func (o DownloadOpts) String() string {
	return fmt.Sprintf("f=%t,b=%s,r=%t,d=%t", o.FetchRemoteIfNeeded, o.BlockForReadUntil.String(), o.RecordOnly, o.CanRedirect)
}

func Execute(ctx rcontext.RequestContext, origin string, mediaId string, opts DownloadOpts) (*database.DbMedia, io.ReadCloser, error) {
	// Step 1: Make our context a timeout context
	var cancel context.CancelFunc
	//goland:noinspection GoVetLostCancel - we handle the function in our custom cancelCloser struct
	ctx.Context, cancel = context.WithTimeout(ctx.Context, opts.BlockForReadUntil)

	// Step 2: Join the singleflight queue for stream and DB record
	sfKey := fmt.Sprintf("%s/%s?%s", origin, mediaId, opts.String())
	fetchRecordFn := func() (*database.DbMedia, error) {
		mediaDb := database.GetInstance().Media.Prepare(ctx)
		record, err := mediaDb.GetById(origin, mediaId)
		if err != nil {
			return nil, err
		}
		if record == nil {
			return download.WaitForAsyncMedia(ctx, origin, mediaId)
		}
		return record, nil
	}
	record, err := recordSf.Do(sfKey, fetchRecordFn)
	defer recordSf.ForgetCacheKey(sfKey)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	r, err, _ := streamSf.Do(sfKey, func() (io.ReadCloser, error) {
		// Step 3: Do we already have the media? Serve it if yes.
		if record != nil {
			if record.Quarantined {
				return quarantine.ReturnAppropriateThing(ctx, true, opts.RecordOnly, 512, 512)
			}
			meta.FlagAccess(ctx, record.Sha256Hash, record.CreationTs)
			if opts.RecordOnly {
				return nil, nil
			}
			if opts.CanRedirect {
				return download.OpenOrRedirect(ctx, record.Locatable)
			} else {
				return download.OpenStream(ctx, record.Locatable)
			}
		}

		// Step 4: Media record unknown - download it (if possible)
		if !opts.FetchRemoteIfNeeded {
			return nil, common.ErrMediaNotFound
		}
		record, r, err := download.TryDownload(ctx, origin, mediaId)
		if err != nil {
			return nil, err
		}
		recordSf.OverwriteCacheKey(sfKey, record)
		if record.Quarantined {
			return quarantine.ReturnAppropriateThing(ctx, true, opts.RecordOnly, 512, 512)
		}
		meta.FlagAccess(ctx, record.Sha256Hash, record.CreationTs)
		if opts.RecordOnly {
			r.Close()
			return nil, nil
		}

		// Step 5: Return the stream
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
	if record == nil {
		// Re-fetch, hopefully from cache
		record, err = recordSf.Do(sfKey, fetchRecordFn)
		if err != nil {
			cancel()
			return nil, nil, err
		}
		if record == nil {
			cancel()
			return nil, nil, errors.New("unexpected error: no viable record and no error condition")
		}
	}
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
