package p

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/url_previewing/m"
	"github.com/turt2live/matrix-media-repo/url_previewing/u"

	"github.com/PuerkitoBio/goquery"
	"github.com/dyatlov/go-opengraph/opengraph"
	ogimage "github.com/dyatlov/go-opengraph/opengraph/types/image"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/metrics"
)

var ogSupportedTypes = []string{"text/*"}

func GenerateOpenGraphPreview(urlPayload *m.UrlPayload, languageHeader string, ctx rcontext.RequestContext) (m.PreviewResult, error) {
	html, err := u.DownloadHtmlContent(urlPayload, ogSupportedTypes, languageHeader, ctx)
	if err != nil {
		ctx.Log.Error("Error downloading content: ", err)

		// Make sure the unsupported error gets passed through
		if err == m.ErrPreviewUnsupported {
			return m.PreviewResult{}, m.ErrPreviewUnsupported
		}

		// We'll consider it not found for the sake of processing
		return m.PreviewResult{}, common.ErrMediaNotFound
	}

	og := opengraph.NewOpenGraph()
	err = og.ProcessHTML(strings.NewReader(html))
	if err != nil {
		ctx.Log.Error("Error getting OpenGraph: ", err)
		return m.PreviewResult{}, err
	}

	if og.Title == "" {
		og.Title = calcTitle(html)
	}
	if og.Description == "" {
		og.Description = calcDescription(html)
	}
	if len(og.Images) == 0 {
		og.Images = calcImages(html)
	}

	// Be sure to trim the title and description
	og.Title = u.Summarize(og.Title, ctx.Config.UrlPreviews.NumTitleWords, ctx.Config.UrlPreviews.MaxTitleLength)
	og.Description = u.Summarize(og.Description, ctx.Config.UrlPreviews.NumWords, ctx.Config.UrlPreviews.MaxLength)

	graph := &m.PreviewResult{
		Type:        og.Type,
		Url:         og.URL,
		Title:       og.Title,
		Description: og.Description,
		SiteName:    og.SiteName,
	}

	if og.Images != nil && len(og.Images) > 0 {
		imgUrl, err := url.Parse(og.Images[0].URL)
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

	metrics.UrlPreviewsGenerated.With(prometheus.Labels{"type": "opengraph"}).Inc()
	return *graph, nil
}

func calcTitle(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}

	titleText := doc.Find("title").First().Text()
	if titleText != "" {
		return titleText
	}

	h1Text := doc.Find("h1").First().Text()
	if h1Text != "" {
		return h1Text
	}

	h2Text := doc.Find("h2").First().Text()
	if h2Text != "" {
		return h2Text
	}

	h3Text := doc.Find("h3").First().Text()
	if h3Text != "" {
		return h3Text
	}

	return ""
}

func calcDescription(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}

	metaDescription, exists := doc.Find("meta[name='description']").First().Attr("content")
	if exists && metaDescription != "" {
		return metaDescription
	}

	// Try and generate a plain text version of the page
	// We remove tags that are probably not going to help
	doc.Find("header").Remove()
	doc.Find("nav").Remove()
	doc.Find("aside").Remove()
	doc.Find("footer").Remove()
	doc.Find("noscript").Remove()
	doc.Find("script").Remove()
	doc.Find("style").Remove()
	return doc.Find("body").Text()
}

func calcImages(html string) []*ogimage.Image {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return []*ogimage.Image{}
	}

	imageSrc := ""
	dimensionScore := 0
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if !exists || src == "" {
			return
		}

		wStr, exists := s.Attr("width")
		if !exists {
			return
		}

		hStr, exists := s.Attr("height")
		if !exists {
			return
		}

		w, _ := strconv.Atoi(wStr)
		h, _ := strconv.Atoi(hStr)

		if w < 10 || h < 10 {
			return // too small
		}

		if (w*h) < dimensionScore || dimensionScore == 0 {
			dimensionScore = w * h
			imageSrc = src
		}
	})

	if imageSrc == "" || dimensionScore <= 0 {
		return []*ogimage.Image{}
	}

	img := ogimage.Image{URL: imageSrc}
	return []*ogimage.Image{&img}
}
