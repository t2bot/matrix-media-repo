package previewers

import (
	"bytes"
	"io/ioutil"
	"net/url"
	"path"

	"github.com/dyatlov/go-oembed/oembed"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/preview_controller/preview_types"
	"github.com/turt2live/matrix-media-repo/metrics"
)

var oembedInstance *oembed.Oembed

func getOembed() *oembed.Oembed {
	if oembedInstance != nil {
		return oembedInstance
	}

	oembedInstance = oembed.NewOembed()

	data, err := ioutil.ReadFile(path.Join(config.Runtime.AssetsPath, "providers.json"))
	if err != nil {
		logrus.Fatal(err)
	}

	err = oembedInstance.ParseProviders(bytes.NewReader(data))
	if err != nil {
		logrus.Fatal(err)
	}

	return oembedInstance
}

func GenerateOEmbedPreview(urlPayload *preview_types.UrlPayload, languageHeader string, ctx rcontext.RequestContext) (preview_types.PreviewResult, error) {
	item := getOembed().FindItem(urlPayload.ParsedUrl.String())
	if item == nil {
		return preview_types.PreviewResult{}, preview_types.ErrPreviewUnsupported
	}

	info, err := item.FetchOembed(oembed.Options{
		URL:            urlPayload.ParsedUrl.String(),
		AcceptLanguage: languageHeader,
	})
	if err != nil {
		ctx.Log.Error("Error getting oEmbed: " + err.Error())
		return preview_types.PreviewResult{}, err
	}

	graph := &preview_types.PreviewResult{
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
			return *graph, nil
		}

		imgAbsUrl := urlPayload.ParsedUrl.ResolveReference(imgUrl)
		imgUrlPayload := &preview_types.UrlPayload{
			UrlString: imgAbsUrl.String(),
			ParsedUrl: imgAbsUrl,
		}

		img, err := downloadImage(imgUrlPayload, languageHeader, ctx)
		if err != nil {
			ctx.Log.Error("Non-fatal error getting thumbnail (downloading image): " + err.Error())
			return *graph, nil
		}

		graph.Image = img
	}

	metrics.UrlPreviewsGenerated.With(prometheus.Labels{"type": "oembed"}).Inc()
	return *graph, nil
}
