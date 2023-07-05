package p

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/metrics"
	"github.com/turt2live/matrix-media-repo/thumbnailing"
	"github.com/turt2live/matrix-media-repo/url_previewing/m"
	"github.com/turt2live/matrix-media-repo/url_previewing/u"
)

func GenerateCalculatedPreview(urlPayload *m.UrlPayload, languageHeader string, ctx rcontext.RequestContext) (m.PreviewResult, error) {
	r, filename, contentType, err := u.DownloadRawContent(urlPayload, ctx.Config.UrlPreviews.FilePreviewTypes, languageHeader, ctx)
	if err != nil {
		ctx.Log.Warn("Error downloading content: ", err)

		// Make sure the unsupported error gets passed through
		if err == m.ErrPreviewUnsupported {
			return m.PreviewResult{}, m.ErrPreviewUnsupported
		}

		// We'll consider it not found for the sake of processing
		return m.PreviewResult{}, common.ErrMediaNotFound
	}

	img := &m.PreviewImage{
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

	result := &m.PreviewResult{
		Type:        "", // intentionally empty
		Url:         urlPayload.ParsedUrl.String(),
		Title:       u.Summarize(filename, ctx.Config.UrlPreviews.NumTitleWords, ctx.Config.UrlPreviews.MaxTitleLength),
		Description: u.Summarize(description, ctx.Config.UrlPreviews.NumWords, ctx.Config.UrlPreviews.MaxLength),
		SiteName:    "", // intentionally empty
	}

	if thumbnailing.IsSupported(img.ContentType) {
		result.Image = img
	} else {
		defer img.Data.Close()
	}

	metrics.UrlPreviewsGenerated.With(prometheus.Labels{"type": "calculated"}).Inc()
	return *result, nil
}
