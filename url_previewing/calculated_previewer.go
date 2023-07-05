package url_previewing

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/ryanuber/go-glob"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/metrics"
)

func GenerateCalculatedPreview(urlPayload *UrlPayload, languageHeader string, ctx rcontext.RequestContext) (Result, error) {
	r, filename, contentType, err := downloadRawContent(urlPayload, ctx.Config.UrlPreviews.FilePreviewTypes, languageHeader, ctx)
	if err != nil {
		ctx.Log.Error("Error downloading content: ", err)

		// Make sure the unsupported error gets passed through
		if err == ErrPreviewUnsupported {
			return Result{}, ErrPreviewUnsupported
		}

		// We'll consider it not found for the sake of processing
		return Result{}, common.ErrMediaNotFound
	}

	img := &Image{
		Data:        r,
		ContentType: contentType,
		Filename:    filename,
	}

	description := ""
	filename = urlPayload.ParsedUrl.String()
	if img != nil && img.Filename != "" {
		filename = img.Filename
	} else {
		description = urlPayload.ParsedUrl.String()
	}

	// Clear the description so we don't duplicate the URL
	if description == filename {
		description = ""
	}

	result := &Result{
		Type:        "", // intentionally empty
		Url:         urlPayload.ParsedUrl.String(),
		Title:       summarize(filename, ctx.Config.UrlPreviews.NumTitleWords, ctx.Config.UrlPreviews.MaxTitleLength),
		Description: summarize(description, ctx.Config.UrlPreviews.NumWords, ctx.Config.UrlPreviews.MaxLength),
		SiteName:    "", // intentionally empty
	}

	if glob.Glob("image/*", img.ContentType) {
		result.Image = img
	}

	metrics.UrlPreviewsGenerated.With(prometheus.Labels{"type": "calculated"}).Inc()
	return *result, nil
}
