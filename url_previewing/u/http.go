package u

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ryanuber/go-glob"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/url_previewing/m"
	"github.com/t2bot/matrix-media-repo/util"
	"github.com/t2bot/matrix-media-repo/util/readers"
	"golang.org/x/net/proxy"
)

func getProxy(dialer *net.Dialer, ctx rcontext.RequestContext) (proxy.Dialer, error) {
	url, err := url.Parse(ctx.Config.UrlPreviews.ProxyURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing proxy url: %w", err)
	}
	return proxy.FromURL(url, dialer)
}

func doHttpGet(urlPayload *m.UrlPayload, languageHeader string, ctx rcontext.RequestContext) (*http.Response, error) {
	var client *http.Client

	dialer := &net.Dialer{
		Timeout:   time.Duration(ctx.Config.TimeoutSeconds.UrlPreviews) * time.Second,
		KeepAlive: time.Duration(ctx.Config.TimeoutSeconds.UrlPreviews) * time.Second,
	}

	dialContext := func(ctx2 context.Context, network, addr string) (conn net.Conn, e error) {
		if network != "tcp" {
			return nil, errors.New("invalid network: expected tcp")
		}

		safeIp, safePort, err := getSafeAddress(addr, ctx)
		if err != nil {
			return nil, err
		}

		if ctx.Config.UrlPreviews.ProxyURL != "" {
			proxyDialer, err := getProxy(dialer, ctx)
			if err != nil {
				return nil, fmt.Errorf("error creating proxy: %w", err)
			}
			if contextDialer, ok := proxyDialer.(proxy.ContextDialer); ok {
				return contextDialer.DialContext(ctx2, network, net.JoinHostPort(safeIp.String(), safePort))
			} else {
				return nil, errors.New("failed proxy type assertion to ContextDialer")
			}
		}

		return dialer.DialContext(ctx2, network, net.JoinHostPort(safeIp.String(), safePort))
	}

	if ctx.Config.UrlPreviews.UnsafeCertificates {
		ctx.Log.Warn("Ignoring any certificate errors while making request")
		tr := &http.Transport{
			DisableKeepAlives: true,
			DialContext:       dialContext,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			// Based on https://github.com/matrix-org/gomatrixserverlib/blob/51152a681e69a832efcd934b60080b92bc98b286/client.go#L74-L90
			DialTLSContext: func(ctx2 context.Context, network, addr string) (net.Conn, error) {
				var rawconn net.Conn
				var connErr error
				if ctx.Config.UrlPreviews.ProxyURL != "" {
					proxyDialer, err := getProxy(dialer, ctx)
					if err != nil {
						return nil, fmt.Errorf("error creating proxy: %w", err)
					}
					rawconn, connErr = proxyDialer.Dial(network, addr)
				} else {
					rawconn, connErr = net.Dial(network, addr)
				}
				if connErr != nil {
					return nil, connErr
				}
				// Wrap a raw connection ourselves since tls.Dial defaults the SNI
				conn := tls.Client(rawconn, &tls.Config{
					ServerName:         "",
					InsecureSkipVerify: true,
				})
				if err := conn.Handshake(); err != nil {
					return nil, err
				}
				return conn, nil
			},
		}
		client = &http.Client{
			Timeout:   time.Duration(ctx.Config.TimeoutSeconds.UrlPreviews) * time.Second,
			Transport: tr,
		}
	} else {
		client = &http.Client{
			Timeout: time.Duration(ctx.Config.TimeoutSeconds.UrlPreviews) * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives: true,
				DialContext:       dialContext,
			},
		}
	}

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
