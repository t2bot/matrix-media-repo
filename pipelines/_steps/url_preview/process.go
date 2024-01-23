package url_preview

import (
	"errors"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/url_previewing/m"
)

func Process(ctx rcontext.RequestContext, previewUrl string, preview m.PreviewResult, err error, onHost string, userId string, languageHeader string, ts int64) (*database.DbUrlPreview, error) {
	previewDb := database.GetInstance().UrlPreviews.Prepare(ctx)

	if err != nil {
		if errors.Is(err, m.ErrPreviewUnsupported) {
			err = common.ErrMediaNotFound
		}

		if errors.Is(err, common.ErrMediaNotFound) {
			previewDb.InsertError(previewUrl, common.ErrCodeNotFound)
		} else {
			previewDb.InsertError(previewUrl, common.ErrCodeUnknown)
		}
		return nil, err
	} else {
		result := &database.DbUrlPreview{
			Url:            previewUrl,
			ErrorCode:      "",
			BucketTs:       ts, // already bucketed
			SiteUrl:        preview.Url,
			SiteName:       preview.SiteName,
			ResourceType:   preview.Type,
			Description:    preview.Description,
			Title:          preview.Title,
			LanguageHeader: languageHeader,
		}

		// Step 7: Store the thumbnail, if needed
		UploadImage(ctx, preview.Image, onHost, userId, result)

		// Step 8: Insert the record
		err = previewDb.Insert(result)
		if err != nil {
			ctx.Log.Warn("Non-fatal error caching URL preview: ", err)
			sentry.CaptureException(err)
		}

		return result, nil
	}
}
