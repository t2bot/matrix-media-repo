package webserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/alioygur/is"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sebest/xff"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/api/r0"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/metrics"
	"github.com/turt2live/matrix-media-repo/util"
)

type handler struct {
	h          func(r *http.Request, ctx rcontext.RequestContext) interface{}
	action     string
	reqCounter *requestCounter
	ignoreHost bool
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	isUsingForwardedHost := false
	if r.Header.Get("X-Forwarded-Host") != "" && config.Get().General.UseForwardedHost {
		r.Host = r.Header.Get("X-Forwarded-Host")
		isUsingForwardedHost = true
	}
	r.Host = strings.Split(r.Host, ":")[0]

	var raddr string
	if config.Get().General.TrustAnyForward {
		raddr = r.Header.Get("X-Forwarded-For")
	} else {
		raddr = xff.GetRemoteAddr(r)
	}
	if raddr == "" {
		raddr = r.RemoteAddr
	}

	host, _, err := net.SplitHostPort(raddr)
	if err != nil {
		logrus.Error(err)
		host = raddr
	}
	r.RemoteAddr = host

	contextLog := logrus.WithFields(logrus.Fields{
		"method":             r.Method,
		"host":               r.Host,
		"usingForwardedHost": isUsingForwardedHost,
		"resource":           r.URL.Path,
		"contentType":        r.Header.Get("Content-Type"),
		"contentLength":      r.ContentLength,
		"queryString":        util.GetLogSafeQueryString(r),
		"requestId":          h.reqCounter.GetNextId(),
		"remoteAddr":         r.RemoteAddr,
	})
	contextLog.Info("Received request")

	// Send CORS and other basic headers
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; script-src 'none'; plugin-types application/pdf; style-src 'unsafe-inline'; media-src 'self'; object-src 'self';")
	w.Header().Set("Server", "matrix-media-repo")

	// Process response
	var res interface{} = api.AuthFailed()
	if util.IsServerOurs(r.Host) || h.ignoreHost {
		contextLog.Info("Host is valid - processing request")
		cfg := config.GetDomain(r.Host)
		if h.ignoreHost {
			dc := config.DomainConfigFrom(*config.Get())
			cfg = &dc
		}

		// Build a context that can be used throughout the remainder of the app
		// This is kinda annoying, but it's better than trying to pass our own
		// thing throughout the layers.
		ctx := r.Context()
		ctx = context.WithValue(ctx, "mr.logger", contextLog)
		ctx = context.WithValue(ctx, "mr.serverConfig", cfg)
		rctx := rcontext.RequestContext{Context: ctx, Log: contextLog, Config: *cfg}
		r = r.WithContext(rctx)

		metrics.HttpRequests.With(prometheus.Labels{
			"host":   r.Host,
			"action": h.action,
			"method": r.Method,
		}).Inc()
		res = h.h(r, rctx)
		if res == nil {
			res = &api.EmptyResponse{}
		}
	} else {
		metrics.InvalidHttpRequests.With(prometheus.Labels{
			"action": h.action,
			"method": r.Method,
		}).Inc()
		contextLog.Warn("The server name provided in the Host header is not configured, or the request was made directly to the media repo instead of through your reverse proxy. This request is being rejected.")
	}
	if res == nil {
		res = api.InternalServerError("Error processing response")
	}

	switch result := res.(type) {
	case *api.DoNotCacheResponse:
		res = result.Payload
		break
	default:
		w.Header().Set("Cache-Control", "public,max-age=86400,s-maxage=86400")
		break
	}

	htmlRes, isHtml := res.(*api.HtmlResponse)
	if isHtml {
		contextLog.Info(fmt.Sprintf("Replying with result: %T %+v", res, fmt.Sprintf("<%d chars of html>", len(htmlRes.HTML))))
	} else {
		contextLog.Info(fmt.Sprintf("Replying with result: %T %+v", res, res))
	}

	statusCode := http.StatusOK
	switch result := res.(type) {
	case *api.ErrorResponse:
		switch result.InternalCode {
		case common.ErrCodeUnknownToken:
			statusCode = http.StatusUnauthorized
			break
		case common.ErrCodeNotFound:
			statusCode = http.StatusNotFound
			break
		case common.ErrCodeMediaTooLarge:
			statusCode = http.StatusRequestEntityTooLarge
			break
		case common.ErrCodeBadRequest:
			statusCode = http.StatusBadRequest
			break
		case common.ErrCodeMethodNotAllowed:
			statusCode = http.StatusMethodNotAllowed
			break
		default: // Treat as unknown (a generic server error)
			statusCode = http.StatusInternalServerError
			break
		}
		break
	case *r0.DownloadMediaResponse:
		metrics.HttpResponses.With(prometheus.Labels{
			"host":       r.Host,
			"action":     h.action,
			"method":     r.Method,
			"statusCode": strconv.Itoa(http.StatusOK),
		}).Inc()

		textTypes := []string{
			"text/css",
			"text/csv",
			"text/html",
			"text/calendar",
			"text/plain",
			"text/javascript",
			"application/json",
			"application/ld+json",
			"application/rtf",
			"image/svg+xml",
			"text/xml",
		}
		contentType := strings.ToLower(result.ContentType)
		for _, v := range textTypes {
			if contentType == v {
				contentType += "; charset=UTF-8"
				break
			}
		}

		w.Header().Set("Cache-Control", "private, max-age=259200") // 3 days
		w.Header().Set("Content-Type", contentType)
		if result.SizeBytes > 0 {
			w.Header().Set("Content-Length", fmt.Sprint(result.SizeBytes))
		}
		if result.Filename != "" {
			if is.ASCII(result.Filename) {
				w.Header().Set("Content-Disposition", "inline; filename="+url.QueryEscape(result.Filename))
			} else {
				w.Header().Set("Content-Disposition", "inline; filename*=utf-8''"+url.QueryEscape(result.Filename))
			}
		}
		defer result.Data.Close()
		writeResponseData(w, result.Data, result.SizeBytes)
		return // Prevent sending conflicting responses
	case *r0.IdenticonResponse:
		metrics.HttpResponses.With(prometheus.Labels{
			"host":       r.Host,
			"action":     h.action,
			"method":     r.Method,
			"statusCode": strconv.Itoa(http.StatusOK),
		}).Inc()
		w.Header().Set("Cache-Control", "private, max-age=604800") // 7 days
		w.Header().Set("Content-Type", "image/png")
		writeResponseData(w, result.Avatar, 0)
		return // Prevent sending conflicting responses
	case *api.HtmlResponse:
		metrics.HttpResponses.With(prometheus.Labels{
			"host":       r.Host,
			"action":     h.action,
			"method":     r.Method,
			"statusCode": strconv.Itoa(http.StatusOK),
		}).Inc()
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.Header().Set("Content-Security-Policy", "") // We're serving HTML, so take away the CSP
		io.Copy(w, bytes.NewBuffer([]byte(result.HTML)))
		return
	default:
		break
	}

	metrics.HttpResponses.With(prometheus.Labels{
		"host":       r.Host,
		"action":     h.action,
		"method":     r.Method,
		"statusCode": strconv.Itoa(statusCode),
	}).Inc()

	// Order is important: Set headers before sending responses
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(statusCode)

	encoder := json.NewEncoder(w)
	encoder.Encode(res)
}

func writeResponseData(w http.ResponseWriter, s io.Reader, expectedBytes int64) {
	b, err := io.Copy(w, s)
	if err != nil {
		// Should only blow up this request
		panic(err)
	}
	if expectedBytes > 0 && b != expectedBytes {
		// Should only blow up this request
		panic(errors.New("mismatch transfer size"))
	}
}
