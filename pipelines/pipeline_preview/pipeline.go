package pipeline_preview

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/pipelines/_steps/url_preview"
	url_previewers2 "github.com/turt2live/matrix-media-repo/url_previewers"
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
		payload := &url_previewers2.UrlPayload{
			UrlString: previewUrl,
			ParsedUrl: parsedUrl,
		}
		var preview url_previewers2.PreviewResult
		err = url_previewers2.ErrPreviewUnsupported

		// Step 5: Try oEmbed
		if ctx.Config.UrlPreviews.OEmbed {
			ctx.Log.Debug("Trying oEmbed previewer")
			preview, err = url_previewers2.GenerateOEmbedPreview(payload, opts.LanguageHeader, ctx)
		}

		// Step 6: Try OpenGraph
		if err == url_previewers2.ErrPreviewUnsupported {
			ctx.Log.Debug("Trying OpenGraph previewer")
			preview, err = url_previewers2.GenerateOpenGraphPreview(payload, opts.LanguageHeader, ctx)
		}

		// Step 7: Try scraping
		if err == url_previewers2.ErrPreviewUnsupported {
			ctx.Log.Debug("Trying built-in previewer")
			preview, err = url_previewers2.GenerateCalculatedPreview(payload, opts.LanguageHeader, ctx)
		}

		// Step 8: Finish processing
		if err != nil {
			if err == url_previewers2.ErrPreviewUnsupported {
				err = common.ErrMediaNotFound
			}

			if err == common.ErrMediaNotFound {
				previewDb.InsertError(previewUrl, common.ErrCodeNotFound)
			} else {
				previewDb.InsertError(previewUrl, common.ErrCodeUnknown)
			}
			return nil, err
		} else {
			result := &database.DbUrlPreview{
				Url:            previewUrl,
				ErrorCode:      "",
				BucketTs:       util.GetHourBucket(opts.Timestamp),
				SiteUrl:        preview.Url,
				SiteName:       preview.SiteName,
				ResourceType:   preview.Type,
				Description:    preview.Description,
				Title:          preview.Title,
				LanguageHeader: opts.LanguageHeader,
			}

			// Step 9: Store the thumbnail, if needed
			url_preview.UploadImage(ctx, preview.Image, onHost, userId, result)

			// Step 10: Insert the record
			err = previewDb.Insert(result)
			if err != nil {
				ctx.Log.Warn("Non-fatal error caching URL preview: ", err)
				sentry.CaptureException(err)
			}

			return result, nil
		}
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
