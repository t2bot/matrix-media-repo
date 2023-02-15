package _routers

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/turt2live/matrix-media-repo/metrics"
)

type MetricsResponseRouter struct {
	next http.Handler
}

func NewMetricsResponseRouter(next http.Handler) *MetricsResponseRouter {
	return &MetricsResponseRouter{next: next}
}

func (m *MetricsResponseRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	metrics.HttpResponses.With(prometheus.Labels{
		"host":       r.Host,
		"action":     GetActionName(r),
		"method":     r.Method,
		"statusCode": strconv.Itoa(GetStatusCode(r)),
	}).Inc()

	if m.next != nil {
		m.next.ServeHTTP(w, r)
	}
}
