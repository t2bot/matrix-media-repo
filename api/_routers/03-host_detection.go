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
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/metrics"
	"github.com/turt2live/matrix-media-repo/util"
)

const domainConfigCtxKey = "mmr.domain_config"

type HostRouter struct {
	next http.Handler
}

func NewHostRouter(next http.Handler) *HostRouter {
	return &HostRouter{next: next}
}

func (h *HostRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Forwarded-Host") != "" && config.Get().General.UseForwardedHost {
		r.Host = r.Header.Get("X-Forwarded-Host")
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
		sentry.CaptureException(err)
		host = raddr
	}
	r.RemoteAddr = host

	ignoreHost := ShouldIgnoreHost(r)
	isOurs := ignoreHost || util.IsServerOurs(r.Host)
	if !isOurs {
		logger := GetLogger(r)
		metrics.InvalidHttpRequests.With(prometheus.Labels{
			"action": GetActionName(r),
			"method": r.Method,
		}).Inc()
		logger.Warn("The server name provided in the Host header is not configured, or the request was made directly to the media repo. Please specify a Host header and check your reverse proxy configuration. The request is being rejected.")
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

	cfg := config.GetDomain(r.Host)
	if ignoreHost {
		dc := config.DomainConfigFrom(*config.Get())
		cfg = &dc
	}

	ctx := r.Context()
	ctx = context.WithValue(ctx, domainConfigCtxKey, cfg)
	r = r.WithContext(ctx)

	if h.next != nil {
		h.next.ServeHTTP(w, r)
	}
}

func GetDomainConfig(r *http.Request) *config.DomainRepoConfig {
	x, ok := r.Context().Value(domainConfigCtxKey).(*config.DomainRepoConfig)
	if !ok {
		return nil
	}
	return x
}
