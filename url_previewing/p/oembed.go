package p

import (
	"bytes"
	"net/url"
	"os"
	"path"
	"sync"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/url_previewing/m"
	"github.com/turt2live/matrix-media-repo/url_previewing/u"

	"github.com/dyatlov/go-oembed/oembed"
	"github.com/k3a/html2text"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/metrics"
)

var oembedInstance *oembed.Oembed
var oembedLock = new(sync.Once)

func getOembed() *oembed.Oembed {
	if oembedInstance != nil {
		return oembedInstance
	}

	oembedLock.Do(func() {
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
	})

	return oembedInstance
}

func GenerateOEmbedPreview(urlPayload *m.UrlPayload, languageHeader string, ctx rcontext.RequestContext) (m.PreviewResult, error) {
	item := getOembed().FindItem(urlPayload.ParsedUrl.String())
	if item == nil {
		return m.PreviewResult{}, m.ErrPreviewUnsupported
	}

	info, err := item.FetchOembed(oembed.Options{
		URL:            urlPayload.ParsedUrl.String(),
		AcceptLanguage: languageHeader,
	})
	if err != nil {
		ctx.Log.Error("Error getting oEmbed: ", err)
		return m.PreviewResult{}, err
	}

	if info.Type == "rich" {
		info.Description = html2text.HTML2Text(info.HTML)
	} else if info.Type == "photo" {
		info.ThumbnailURL = info.URL
	}

	graph := &m.PreviewResult{
		Type:        info.Type,
		Url:         info.URL,
		Title:       info.Title,
		Description: info.Description,
		SiteName:    info.ProviderName,
	}

	if info.ThumbnailURL != "" {
		imgUrl, err := url.Parse(info.ThumbnailURL)
		if err != nil {
			ctx.Log.Error("Non-fatal error getting thumbnail (parsing image url): ", err)
			sentry.CaptureException(err)
			return *graph, nil
		}

		imgAbsUrl := urlPayload.ParsedUrl.ResolveReference(imgUrl)
		imgUrlPayload := &m.UrlPayload{
			UrlString: imgAbsUrl.String(),
			ParsedUrl: imgAbsUrl,
		}

		img, err := u.DownloadImage(imgUrlPayload, languageHeader, ctx)
		if err != nil {
			ctx.Log.Error("Non-fatal error getting thumbnail (downloading image): ", err)
			sentry.CaptureException(err)
			return *graph, nil
		}

		graph.Image = img
	}

	metrics.UrlPreviewsGenerated.With(prometheus.Labels{"type": "oembed"}).Inc()
	return *graph, nil
}
