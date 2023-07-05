package url_preview

import (
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/url_previewing/m"
	"github.com/turt2live/matrix-media-repo/url_previewing/p"
)

func Preview(ctx rcontext.RequestContext, targetUrl *m.UrlPayload, languageHeader string) (m.PreviewResult, error) {
	var preview m.PreviewResult
	err := m.ErrPreviewUnsupported

	// Try oEmbed first
	if ctx.Config.UrlPreviews.OEmbed {
		ctx.Log.Debug("Trying oEmbed previewer")
		preview, err = p.GenerateOEmbedPreview(targetUrl, languageHeader, ctx)
	}

	// Try OpenGraph if that failed
	if err == m.ErrPreviewUnsupported {
		ctx.Log.Debug("Trying OpenGraph previewer")
		preview, err = p.GenerateOpenGraphPreview(targetUrl, languageHeader, ctx)
	}

	// Try scraping if that failed
	if err == m.ErrPreviewUnsupported {
		ctx.Log.Debug("Trying built-in previewer")
		preview, err = p.GenerateCalculatedPreview(targetUrl, languageHeader, ctx)
	}

	return preview, err
}
