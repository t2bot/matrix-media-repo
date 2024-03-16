package routers

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/t2bot/matrix-media-repo/metrics"
)

type MetricsRequestRouter struct {
	next http.Handler
}

func NewMetricsRequestRouter(next http.Handler) *MetricsRequestRouter {
	return &MetricsRequestRouter{next: next}
}

func (m *MetricsRequestRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	metrics.HttpRequests.With(prometheus.Labels{
		"host":   r.Host,
		"action": GetActionName(r),
		"method": r.Method,
	}).Inc()

	if m.next != nil {
		m.next.ServeHTTP(w, r)
	}
}
