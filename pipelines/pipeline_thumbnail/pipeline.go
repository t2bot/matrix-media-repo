package pipeline_thumbnail

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/go-leaky-bucket"
	sfstreams "github.com/t2bot/go-singleflight-streams"

	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/limits"
	"github.com/t2bot/matrix-media-repo/pipelines/_steps/download"
	"github.com/t2bot/matrix-media-repo/pipelines/_steps/quarantine"
	"github.com/t2bot/matrix-media-repo/pipelines/_steps/thumbnails"
	"github.com/t2bot/matrix-media-repo/pipelines/pipeline_download"
	"github.com/t2bot/matrix-media-repo/restrictions"
	"github.com/t2bot/matrix-media-repo/util/readers"
	"github.com/t2bot/matrix-media-repo/util/sfcache"
)

var streamSf = new(sfstreams.Group)
var recordSf = sfcache.NewSingleflightCache[*database.DbThumbnail]()

func init() {
	streamSf.UseSeekers = true
}

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
		AuthProvided:        o.AuthProvided,
	}
}

func Execute(ctx rcontext.RequestContext, origin string, mediaId string, opts ThumbnailOpts) (*database.DbThumbnail, io.ReadCloser, error) {
	// Step 0: Check restrictions
	if requiresAuth, err := restrictions.DoesMediaRequireAuth(ctx, origin, mediaId); err != nil {
		return nil, nil, err
	} else if requiresAuth && !opts.AuthProvided {
		return nil, nil, common.ErrRestrictedAuth
	}

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

	// Step 3: Join the singleflight queue for stream and DB record
	sfKey := fmt.Sprintf("%s/%s?%s", origin, mediaId, opts.String())
	fetchRecordFn := func() (*database.DbThumbnail, error) {
		thumbDb := database.GetInstance().Thumbnails.Prepare(ctx)
		return thumbDb.GetByParams(origin, mediaId, opts.Width, opts.Height, opts.Method, opts.Animated)
	}
	record, err := recordSf.Do(sfKey, fetchRecordFn)
	defer recordSf.ForgetCacheKey(sfKey)
	if err != nil {
		cancel()
		return nil, nil, err
	}

	// Check rate limits before moving on much further
	subject := limits.GetRequestIP(ctx.Request)
	limitBucket, err := limits.GetBucket(ctx, subject)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	if limitBucket != nil && record != nil && !opts.RecordOnly {
		if limitErr := limitBucket.Add(record.SizeBytes); limitErr != nil {
			cancel()
			if errors.Is(limitErr, leaky.ErrBucketFull) {
				ctx.Log.WithField("subject", subject).
					Debugf("Rate limited on SizeBytes=%d/%d", record.SizeBytes, limitBucket.Remaining())
				return nil, nil, common.ErrRateLimitExceeded
			}
			return nil, nil, limitErr
		}
	}

	r, err, _ := streamSf.Do(sfKey, func() (io.ReadCloser, error) {
		// Step 4: Get the associated media record (without stream)
		mediaRecord, dr, err := pipeline_download.Execute(ctx, origin, mediaId, opts.ImpliedDownloadOpts())
		if dr != nil {
			// Shouldn't be returned, but just in case...
			dr.Close()
		}
		if err != nil {
			if errors.Is(err, common.ErrMediaQuarantined) {
				recordSf.OverwriteCacheKey(sfKey, nil) // force record to be nil (not found)
				return quarantine.ReturnAppropriateThing(ctx, false, opts.RecordOnly, opts.Width, opts.Height)
			}
			return nil, err
		}
		if mediaRecord == nil {
			return nil, common.ErrMediaNotFound
		}

		// Step 5: See if we're lucky enough to already have this thumbnail
		// Dev note: we already checked above, but there's a small chance we raced with another singleflight, so
		// check again if the original record was nil
		if record == nil {
			record, err = recordSf.Do(sfKey, fetchRecordFn)
			if err != nil {
				return nil, err
			}
		}
		if record != nil {
			if opts.RecordOnly {
				return nil, nil
			}
			if opts.CanRedirect {
				return download.OpenOrRedirect(ctx, record.Locatable)
			} else {
				return download.OpenStream(ctx, record.Locatable)
			}
		}

		// Step 6: Generate the thumbnail and return that
		record, r, err := thumbnails.Generate(ctx, mediaRecord, opts.Width, opts.Height, opts.Method, opts.Animated)
		if err != nil {
			if !opts.RecordOnly && errors.Is(err, common.ErrMediaDimensionsTooSmall) {
				var d io.ReadSeekCloser
				if opts.CanRedirect {
					d, err = download.OpenOrRedirect(ctx, mediaRecord.Locatable)
				} else {
					d, err = download.OpenStream(ctx, mediaRecord.Locatable)
				}
				if err != nil {
					return nil, err
				} else {
					return d, common.ErrMediaDimensionsTooSmall
				}
			}
			return nil, err
		}
		recordSf.OverwriteCacheKey(sfKey, record)
		if opts.RecordOnly {
			defer r.Close()
			return nil, nil
		}

		// Step 7: Return stream
		return r, nil
	})
	if errors.Is(err, common.ErrMediaQuarantined) || errors.Is(err, common.ErrMediaDimensionsTooSmall) {
		if r != nil {
			return nil, readers.NewCancelCloser(r, cancel), err
		}

		if limitBucket != nil {
			if limitErr := limitBucket.Drain(record.SizeBytes); limitErr != nil {
				sentry.CaptureException(limitErr)
				ctx.Log.Warn("Non-fatal error during bucket drain:", limitErr)
			}
		}

		cancel()
		return nil, nil, err
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
