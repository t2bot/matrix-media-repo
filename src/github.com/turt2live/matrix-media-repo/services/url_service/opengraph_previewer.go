package url_service

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dyatlov/go-opengraph/opengraph"
	"github.com/ryanuber/go-glob"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/config"
)

var ogSupportedTypes = []string{"text/*"}

type openGraphUrlPreviewer struct {
	ctx context.Context
	log *logrus.Entry
}

func NewOpenGraphPreviewer(ctx context.Context, log *logrus.Entry) *openGraphUrlPreviewer {
	return &openGraphUrlPreviewer{ctx, log}
}

func (p *openGraphUrlPreviewer) GeneratePreview(urlStr string) (previewResult, error) {
	html, err := downloadHtmlContent(urlStr, p.log)
	if err != nil {
		p.log.Error("Error downloading content: " + err.Error())

		// Make sure the unsupported error gets passed through
		if err == ErrPreviewUnsupported {
			return previewResult{}, ErrPreviewUnsupported
		}

		// We'll consider it not found for the sake of processing
		return previewResult{}, common.ErrMediaNotFound
	}

	og := opengraph.NewOpenGraph()
	err = og.ProcessHTML(strings.NewReader(html))
	if err != nil {
		p.log.Error("Error getting OpenGraph: " + err.Error())
		return previewResult{}, err
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
	og.Title = summarize(og.Title, config.Get().UrlPreviews.NumTitleWords, config.Get().UrlPreviews.MaxTitleLength)
	og.Description = summarize(og.Description, config.Get().UrlPreviews.NumWords, config.Get().UrlPreviews.MaxLength)

	graph := &previewResult{
		Type:        og.Type,
		Url:         og.URL,
		Title:       og.Title,
		Description: og.Description,
		SiteName:    og.SiteName,
	}

	if og.Images != nil && len(og.Images) > 0 {
		baseUrl, err := url.Parse(urlStr)
		if err != nil {
			p.log.Error("Non-fatal error getting thumbnail (parsing base url): " + err.Error())
			return *graph, nil
		}

		imgUrl, err := url.Parse(og.Images[0].URL)
		if err != nil {
			p.log.Error("Non-fatal error getting thumbnail (parsing image url): " + err.Error())
			return *graph, nil
		}

		imgAbsUrl := baseUrl.ResolveReference(imgUrl)
		img, err := downloadImage(imgAbsUrl.String(), p.log)
		if err != nil {
			p.log.Error("Non-fatal error getting thumbnail (downloading image): " + err.Error())
			return *graph, nil
		}

		graph.Image = img
	}

	return *graph, nil
}

func downloadHtmlContent(urlStr string, log *logrus.Entry) (string, error) {
	log.Info("Fetching remote content...")
	resp, err := http.Get(urlStr)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		log.Warn("Received status code " + strconv.Itoa(resp.StatusCode))
		return "", errors.New("error during transfer")
	}

	if config.Get().UrlPreviews.MaxPageSizeBytes > 0 && resp.ContentLength >= 0 && resp.ContentLength > config.Get().UrlPreviews.MaxPageSizeBytes {
		return "", common.ErrMediaTooLarge
	}

	var reader io.Reader
	reader = resp.Body
	if config.Get().UrlPreviews.MaxPageSizeBytes > 0 {
		reader = io.LimitReader(resp.Body, config.Get().UrlPreviews.MaxPageSizeBytes)
	}

	bytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", err
	}

	html := string(bytes)
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	for _, supportedType := range ogSupportedTypes {
		if !glob.Glob(supportedType, contentType) {
			return "", ErrPreviewUnsupported
		}
	}

	return html, nil
}

func downloadImage(imageUrl string, log *logrus.Entry) (*previewImage, error) {
	log.Info("Getting image from " + imageUrl)
	resp, err := http.Get(imageUrl)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		log.Warn("Received status code " + strconv.Itoa(resp.StatusCode))
		return nil, errors.New("error during transfer")
	}

	image := &previewImage{
		ContentType:         resp.Header.Get("Content-Type"),
		Data:                resp.Body,
		ContentLength:       resp.ContentLength,
		ContentLengthHeader: resp.Header.Get("Content-Length"),
	}

	_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition"))
	if err == nil && params["filename"] != "" {
		image.Filename = params["filename"]
	}

	return image, nil
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

func calcImages(html string) []*opengraph.Image {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return []*opengraph.Image{}
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
		return []*opengraph.Image{}
	}

	img := opengraph.Image{URL: imageSrc}
	return []*opengraph.Image{&img}
}
