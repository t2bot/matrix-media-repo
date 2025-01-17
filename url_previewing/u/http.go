package u

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ryanuber/go-glob"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/matrix"
	"github.com/t2bot/matrix-media-repo/url_previewing/m"
	"github.com/t2bot/matrix-media-repo/util"
	"github.com/t2bot/matrix-media-repo/util/readers"
)

func doHttpGet(urlPayload *m.UrlPayload, languageHeader string, ctx rcontext.RequestContext) (*http.Response, error) {
	if urlPayload.ParsedUrl.Scheme != "https" {
		return nil, errors.New("must provide https url")
	}

	var proxyUrlProvider func(*http.Request) (*url.URL, error)
	if ctx.Config.UrlPreviews.ProxyURL != "" {
		proxyUrl, err := url.Parse(ctx.Config.UrlPreviews.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("error parsing proxy url: %w", err)
		}
		proxyUrlProvider = http.ProxyURL(proxyUrl)
	}

	client := matrix.NewHttpClient(ctx, &matrix.HttpClientConfig{
		Timeout:                time.Duration(ctx.Config.TimeoutSeconds.UrlPreviews) * time.Second,
		AllowUnsafeCertificate: ctx.Config.UrlPreviews.UnsafeCertificates,
		AllowedCIDRs:           ctx.Config.UrlPreviews.AllowedNetworks,
		DeniedCIDRs:            ctx.Config.UrlPreviews.DisallowedNetworks,
		FollowRedirects:        true, // we may need to chase some resources down
		Proxy:                  proxyUrlProvider,
	})

	req, err := http.NewRequest("GET", urlPayload.ParsedUrl.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ctx.Config.UrlPreviews.UserAgent)
	req.Header.Set("Accept-Language", languageHeader)
	return client.Do(req)
}

func DownloadRawContent(urlPayload *m.UrlPayload, supportedTypes []string, languageHeader string, ctx rcontext.RequestContext) (io.ReadCloser, string, string, error) {
	ctx.Log.Info("Fetching remote content...")
	resp, err := doHttpGet(urlPayload, languageHeader, ctx)
	if err != nil {
		return nil, "", "", err
	}
	if resp.StatusCode != http.StatusOK {
		ctx.Log.Warn("Received status code " + strconv.Itoa(resp.StatusCode))
		return nil, "", "", errors.New("error during transfer")
	}

	if ctx.Config.UrlPreviews.MaxPageSizeBytes > 0 && resp.ContentLength >= 0 && resp.ContentLength > ctx.Config.UrlPreviews.MaxPageSizeBytes {
		return nil, "", "", common.ErrMediaTooLarge
	}

	var reader io.ReadCloser
	if ctx.Config.UrlPreviews.MaxPageSizeBytes > 0 {
		lr := io.LimitReader(resp.Body, ctx.Config.UrlPreviews.MaxPageSizeBytes)
		reader = readers.NewCancelCloser(io.NopCloser(lr), func() {
			resp.Body.Close()
		})
	}

	contentType := resp.Header.Get("Content-Type")
	for _, supportedType := range supportedTypes {
		if !glob.Glob(supportedType, contentType) {
			return nil, "", "", m.ErrPreviewUnsupported
		}
	}

	disposition := resp.Header.Get("Content-Disposition")
	_, params, _ := mime.ParseMediaType(disposition)
	filename := ""
	if params != nil {
		filename = params["filename"]
	}

	return reader, filename, contentType, nil
}

func DownloadHtmlContent(urlPayload *m.UrlPayload, supportedTypes []string, languageHeader string, ctx rcontext.RequestContext) (string, error) {
	r, _, contentType, err := DownloadRawContent(urlPayload, supportedTypes, languageHeader, ctx)
	if err != nil {
		return "", err
	}
	html := ""
	defer r.Close()
	raw, _ := io.ReadAll(r)
	if raw != nil {
		html = util.ToUtf8(string(raw), contentType)
	}
	return html, nil
}

func DownloadImage(urlPayload *m.UrlPayload, languageHeader string, ctx rcontext.RequestContext) (*m.PreviewImage, error) {
	ctx.Log.Info("Getting image from " + urlPayload.ParsedUrl.String())
	resp, err := doHttpGet(urlPayload, languageHeader, ctx)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		ctx.Log.Warn("Received status code " + strconv.Itoa(resp.StatusCode))
		return nil, errors.New("error during transfer")
	}

	image := &m.PreviewImage{
		ContentType: resp.Header.Get("Content-Type"),
		Data:        resp.Body,
	}

	_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition"))
	if err == nil && params["filename"] != "" {
		image.Filename = params["filename"]
	}

	return image, nil
}
