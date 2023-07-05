package pipeline_preview

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/pipelines/_steps/url_preview"
	"github.com/turt2live/matrix-media-repo/url_previewing/m"
	"github.com/turt2live/matrix-media-repo/util"
	"golang.org/x/sync/singleflight"
)

var sf = new(singleflight.Group)

type PreviewOpts struct {
	Timestamp      int64
	LanguageHeader string
}

func Execute(ctx rcontext.RequestContext, onHost string, previewUrl string, userId string, opts PreviewOpts) (*database.DbUrlPreview, error) {
	// Step 1: Check database cache
	previewDb := database.GetInstance().UrlPreviews.Prepare(ctx)
	record, err := previewDb.Get(previewUrl, opts.Timestamp, opts.LanguageHeader)
	if err != nil || record != nil {
		return record, err
	}

	// Step 2: Fix timestamp bucket. If we're within 60 seconds of a bucket, just assume we're okay, so we don't
	// infinitely recurse into ourselves.
	now := util.NowMillis()
	atBucket := util.GetHourBucket(opts.Timestamp)
	nowBucket := util.GetHourBucket(now)
	if (now-opts.Timestamp) > 60000 && atBucket != nowBucket {
		return Execute(ctx, onHost, previewUrl, userId, PreviewOpts{
			Timestamp:      now,
			LanguageHeader: opts.LanguageHeader,
		})
	}

	// Step 3: Process the URL
	parsedUrl, err := url.Parse(previewUrl)
	if err != nil {
		previewDb.InsertError(previewUrl, common.ErrCodeInvalidHost)
		return nil, common.ErrInvalidHost
	}
	parsedUrl.Fragment = "" // remove fragments because they're not useful to servers

	// Step 4: Join the singleflight queue
	r, err, _ := sf.Do(fmt.Sprintf("%s:%s_%d/%s", onHost, previewUrl, opts.Timestamp, opts.LanguageHeader), func() (interface{}, error) {
		// Step 5: Generate preview
		var preview m.PreviewResult
		preview, err = url_preview.Preview(ctx, &m.UrlPayload{
			UrlString: previewUrl,
			ParsedUrl: parsedUrl,
		}, opts.LanguageHeader)

		// Step 6: Finish processing
		return url_preview.Process(ctx, previewUrl, preview, err, onHost, userId, opts.LanguageHeader, opts.Timestamp)
	})
	if err != nil {
		return nil, err
	}
	if val, ok := r.(*database.DbUrlPreview); !ok {
		return nil, errors.New("runtime error: expected DbUrlPreview, got something else")
	} else {
		return val, nil
	}
}
