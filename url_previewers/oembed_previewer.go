package url_previewers

import (
	"bytes"
	"net/url"
	"os"
	"path"

	"github.com/getsentry/sentry-go"

	"github.com/dyatlov/go-oembed/oembed"
	"github.com/k3a/html2text"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/metrics"
)

var oembedInstance *oembed.Oembed

func getOembed() *oembed.Oembed {
	if oembedInstance != nil {
		return oembedInstance
	}

	oembedInstance = oembed.NewOembed()

	data, err := os.ReadFile(path.Join(config.Runtime.AssetsPath, "providers.json"))
	if err != nil {
		sentry.CaptureException(err)
		logrus.Fatal(err)
	}

	err = oembedInstance.ParseProviders(bytes.NewReader(data))
	if err != nil {
		sentry.CaptureException(err)
		logrus.Fatal(err)
	}

	return oembedInstance
}

func GenerateOEmbedPreview(urlPayload *UrlPayload, languageHeader string, ctx rcontext.RequestContext) (PreviewResult, error) {
	item := getOembed().FindItem(urlPayload.ParsedUrl.String())
	if item == nil {
		return PreviewResult{}, ErrPreviewUnsupported
	}

	info, err := item.FetchOembed(oembed.Options{
		URL:            urlPayload.ParsedUrl.String(),
		AcceptLanguage: languageHeader,
	})
	if err != nil {
		ctx.Log.Error("Error getting oEmbed: " + err.Error())
		return PreviewResult{}, err
	}

	if info.Type == "rich" {
		info.Description = html2text.HTML2Text(info.HTML)
	} else if info.Type == "photo" {
		info.ThumbnailURL = info.URL
	}

	graph := &PreviewResult{
		Type:        info.Type,
		Url:         info.URL,
		Title:       info.Title,
		Description: info.Description,
		SiteName:    info.ProviderName,
	}

	if info.ThumbnailURL != "" {
		imgUrl, err := url.Parse(info.ThumbnailURL)
		if err != nil {
			ctx.Log.Error("Non-fatal error getting thumbnail (parsing image url): " + err.Error())
			sentry.CaptureException(err)
			return *graph, nil
		}

		imgAbsUrl := urlPayload.ParsedUrl.ResolveReference(imgUrl)
		imgUrlPayload := &UrlPayload{
			UrlString: imgAbsUrl.String(),
			ParsedUrl: imgAbsUrl,
		}

		img, err := downloadImage(imgUrlPayload, languageHeader, ctx)
		if err != nil {
			ctx.Log.Error("Non-fatal error getting thumbnail (downloading image): " + err.Error())
			sentry.CaptureException(err)
			return *graph, nil
		}

		graph.Image = img
	}

	metrics.UrlPreviewsGenerated.With(prometheus.Labels{"type": "oembed"}).Inc()
	return *graph, nil
}
