package previewers

import (
	"crypto/tls"
	"errors"
	"io"
	"io/ioutil"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/dyatlov/go-opengraph/opengraph"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/ryanuber/go-glob"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/metrics"
)

var ogSupportedTypes = []string{"text/*"}

func GenerateOpenGraphPreview(urlStr string, log *logrus.Entry) (PreviewResult, error) {
	html, err := downloadHtmlContent(urlStr, log)
	if err != nil {
		log.Error("Error downloading content: " + err.Error())

		// Make sure the unsupported error gets passed through
		if err == ErrPreviewUnsupported {
			return PreviewResult{}, ErrPreviewUnsupported
		}

		// We'll consider it not found for the sake of processing
		return PreviewResult{}, common.ErrMediaNotFound
	}

	og := opengraph.NewOpenGraph()
	err = og.ProcessHTML(strings.NewReader(html))
	if err != nil {
		log.Error("Error getting OpenGraph: " + err.Error())
		return PreviewResult{}, err
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

	graph := &PreviewResult{
		Type:        og.Type,
		Url:         og.URL,
		Title:       og.Title,
		Description: og.Description,
		SiteName:    og.SiteName,
	}

	if og.Images != nil && len(og.Images) > 0 {
		baseUrl, err := url.Parse(urlStr)
		if err != nil {
			log.Error("Non-fatal error getting thumbnail (parsing base url): " + err.Error())
			return *graph, nil
		}

		imgUrl, err := url.Parse(og.Images[0].URL)
		if err != nil {
			log.Error("Non-fatal error getting thumbnail (parsing image url): " + err.Error())
			return *graph, nil
		}

		imgAbsUrl := baseUrl.ResolveReference(imgUrl)
		img, err := downloadImage(imgAbsUrl.String(), log)
		if err != nil {
			log.Error("Non-fatal error getting thumbnail (downloading image): " + err.Error())
			return *graph, nil
		}

		graph.Image = img
	}

	metrics.UrlPreviewsGenerated.With(prometheus.Labels{"type": "opengraph"}).Inc()
	return *graph, nil
}

func doHttpGet(urlStr string, log *logrus.Entry) (*http.Response, error) {
	var client *http.Client
	if config.Get().UrlPreviews.UnsafeCertificates {
		log.Warn("Ignoring any certificate errors while making request")
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			// Based on https://github.com/matrix-org/gomatrixserverlib/blob/51152a681e69a832efcd934b60080b92bc98b286/client.go#L74-L90
			DialTLS: func(network, addr string) (net.Conn, error) {
				rawconn, err := net.Dial(network, addr)
				if err != nil {
					return nil, err
				}
				// Wrap a raw connection ourselves since tls.Dial defaults the SNI
				conn := tls.Client(rawconn, &tls.Config{
					ServerName: "",
					// TODO: We should be checking that the TLS certificate we see here matches one of the allowed SHA-256 fingerprints for the server.
					InsecureSkipVerify: true,
				})
				if err := conn.Handshake(); err != nil {
					return nil, err
				}
				return conn, nil
			},
		}
		client = &http.Client{
			Transport: tr,
			Timeout:   time.Duration(config.Get().TimeoutSeconds.UrlPreviews) * time.Second,
		}
	} else {
		client = &http.Client{
			Timeout: time.Duration(config.Get().TimeoutSeconds.UrlPreviews) * time.Second,
		}
	}

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "matrix-media-repo")
	return client.Do(req)
}

func downloadHtmlContent(urlStr string, log *logrus.Entry) (string, error) {
	log.Info("Fetching remote content...")
	resp, err := doHttpGet(urlStr, log)
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

func downloadImage(imageUrl string, log *logrus.Entry) (*PreviewImage, error) {
	log.Info("Getting image from " + imageUrl)
	resp, err := doHttpGet(imageUrl, log)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		log.Warn("Received status code " + strconv.Itoa(resp.StatusCode))
		return nil, errors.New("error during transfer")
	}

	image := &PreviewImage{
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
