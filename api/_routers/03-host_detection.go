package _routers

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sebest/xff"
	"github.com/sirupsen/logrus"

	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/metrics"
	"github.com/t2bot/matrix-media-repo/util"
)

type HostRouter struct {
	next http.Handler
}

func NewHostRouter(next http.Handler) *HostRouter {
	return &HostRouter{next: next}
}

func (h *HostRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	origHost := r.Host
	origRemoteAddr := r.RemoteAddr

	if r.Header.Get("X-Forwarded-Host") != "" && config.Get().General.UseForwardedHost {
		r.Host = r.Header.Get("X-Forwarded-Host")
	}
	r.Host = strings.Split(r.Host, ":")[0]
	r.RemoteAddr = GetRemoteAddr(r)

	ignoreHost := ShouldIgnoreHost(r)
	isOurs := ignoreHost || util.IsServerOurs(r.Host)
	if !isOurs {
		logger := GetLogger(r)
		metrics.InvalidHttpRequests.With(prometheus.Labels{
			"action": GetActionName(r),
			"method": r.Method,
		}).Inc()
		logger.Warnf("The server name provided ('%s') in the Host header is not configured, or the request was made directly to the media repo. Please specify a Host header and check your reverse proxy configuration. The request is being rejected.", r.Host)
		w.WriteHeader(http.StatusBadGateway)
		if b, err := json.Marshal(_responses.BadGatewayError("Review server logs to continue")); err != nil {
			panic(errors.New("error preparing BadGatewayError: " + err.Error()))
		} else {
			if _, err = w.Write(b); err != nil {
				panic(errors.New("error sending BadGatewayError: " + err.Error()))
			}
		}
		return // don't call next handler
	}

	logger := GetLogger(r).WithFields(logrus.Fields{
		"host":           r.Host,
		"remoteAddr":     r.RemoteAddr,
		"origHost":       origHost,
		"origRemoteAddr": origRemoteAddr,
	})

	cfg := config.GetDomain(r.Host)
	if ignoreHost {
		dc := config.DomainConfigFrom(*config.Get())
		cfg = &dc
	}

	ctx := r.Context()
	ctx = context.WithValue(ctx, common.ContextDomainConfig, cfg)
	ctx = context.WithValue(ctx, common.ContextLogger, logger)
	r = r.WithContext(ctx)

	if h.next != nil {
		h.next.ServeHTTP(w, r)
	}
}

func GetRemoteAddr(r *http.Request) string {
	if config.Get().General.TrustAnyForward {
		return r.Header.Get("X-Forwarded-For")
	}

	raddr := xff.GetRemoteAddr(r)
	if raddr == "" {
		raddr = r.RemoteAddr
	}

	host, _, err := net.SplitHostPort(raddr)
	if err != nil {
		logrus.WithField("raddr", raddr).WithError(err).Error("Invalid remote address")
		sentry.CaptureException(err)
		host = raddr
	}

	return host
}

func GetDomainConfig(r *http.Request) *config.DomainRepoConfig {
	x, ok := r.Context().Value(common.ContextDomainConfig).(*config.DomainRepoConfig)
	if !ok {
		return nil
	}
	return x
}
