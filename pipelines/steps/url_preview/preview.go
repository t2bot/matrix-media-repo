package url_preview

import (
	"errors"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/pool"
	"github.com/t2bot/matrix-media-repo/url_previewing/m"
	"github.com/t2bot/matrix-media-repo/url_previewing/p"
)

type generateResult struct {
	preview m.PreviewResult
	err     error
}

func Preview(ctx rcontext.RequestContext, targetUrl *m.UrlPayload, languageHeader string) (m.PreviewResult, error) {
	ch := make(chan generateResult)
	defer close(ch)
	fn := func() {
		var preview m.PreviewResult
		err := m.ErrPreviewUnsupported

		// Try oEmbed first
		if ctx.Config.UrlPreviews.OEmbed {
			ctx.Log.Debug("Trying oEmbed previewer")
			preview, err = p.GenerateOEmbedPreview(targetUrl, languageHeader, ctx)
		}

		// Try OpenGraph if that failed
		if errors.Is(err, m.ErrPreviewUnsupported) {
			ctx.Log.Debug("Trying OpenGraph previewer")
			preview, err = p.GenerateOpenGraphPreview(targetUrl, languageHeader, ctx)
		}

		// Try scraping if that failed
		if errors.Is(err, m.ErrPreviewUnsupported) {
			ctx.Log.Debug("Trying built-in previewer")
			preview, err = p.GenerateCalculatedPreview(targetUrl, languageHeader, ctx)
		}

		ch <- generateResult{
			preview: preview,
			err:     err,
		}
	}

	if err := pool.UrlPreviewQueue.Schedule(fn); err != nil {
		return m.PreviewResult{}, err
	}
	res := <-ch
	return res.preview, res.err
}
