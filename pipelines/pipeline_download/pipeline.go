package pipeline_download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/go-leaky-bucket"
	"github.com/t2bot/go-singleflight-streams"

	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/limits"
	"github.com/t2bot/matrix-media-repo/matrix"
	"github.com/t2bot/matrix-media-repo/pipelines/_steps/download"
	"github.com/t2bot/matrix-media-repo/pipelines/_steps/meta"
	"github.com/t2bot/matrix-media-repo/pipelines/_steps/quarantine"
	"github.com/t2bot/matrix-media-repo/restrictions"
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
	AuthProvided        bool
}

func (o DownloadOpts) String() string {
	return fmt.Sprintf("f=%t,b=%s,r=%t,d=%t", o.FetchRemoteIfNeeded, o.BlockForReadUntil.String(), o.RecordOnly, o.CanRedirect)
}

func Execute(ctx rcontext.RequestContext, origin string, mediaId string, opts DownloadOpts) (*database.DbMedia, io.ReadCloser, error) {
	// Step 0: Check restrictions
	if requiresAuth, err := restrictions.DoesMediaRequireAuth(ctx, origin, mediaId); err != nil {
		return nil, nil, err
	} else if requiresAuth && !opts.AuthProvided {
		return nil, nil, common.ErrRestrictedAuth
	}

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

	// Check rate limits before moving on much further
	subject := limits.GetRequestIP(ctx.Request)
	limitBucket, err := limits.GetBucket(ctx, subject)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	didBucketMaxSize := false
	if limitBucket != nil {
		if record == nil {
			if opts.FetchRemoteIfNeeded {
				// No record means we may need to download it. Track that in the bucket.
				// We use the maximum size possible for now, until we actually know the file size.
				if limitErr := limitBucket.Add(ctx.Config.Downloads.MaxSizeBytes); limitErr != nil {
					cancel()
					if errors.Is(limitErr, leaky.ErrBucketFull) {
						ctx.Log.WithField("subject", subject).
							Debugf("Rate limited on MaxSizeBytes=%d/%d", ctx.Config.Downloads.MaxSizeBytes, limitBucket.Remaining())
						return nil, nil, common.ErrRateLimitExceeded
					}
					return nil, nil, limitErr
				}
				didBucketMaxSize = true
			}
		} else if !opts.RecordOnly && !record.Quarantined { // check that a media request body is going to be returned
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
	var notAllowedErr *matrix.ServerNotAllowedError
	if errors.As(err, &notAllowedErr) {
		if notAllowedErr.ServerName != ctx.Request.Host {
			ctx.Log.Debug("'Not allowed' error is for another server - retrying")
			cancel()
			return Execute(ctx, origin, mediaId, opts)
		}
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
	if didBucketMaxSize && limitBucket != nil {
		// We need to restore the difference between max size and actual size to the caller's bucket.
		// If for some reason the downloaded file is larger than the max size, the bucket will be added to instead.
		// We should only get a limit error when the file is larger than the max size.
		if limitErr := limitBucket.Drain(ctx.Config.Downloads.MaxSizeBytes - record.SizeBytes); limitErr != nil {
			cancel()
			if errors.Is(limitErr, leaky.ErrBucketFull) {
				return nil, nil, common.ErrRateLimitExceeded
			}
			return nil, nil, limitErr
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
