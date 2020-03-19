package metrics

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
)

var srv *http.Server

func internalHandler(res http.ResponseWriter, req *http.Request) {
	logrus.Info("Updating live metrics for cache")
	for _, fn := range beforeMetricsCalledFns {
		fn()
	}
	promhttp.Handler().ServeHTTP(res, req)
}

func Init() {
	if !config.Get().Metrics.Enabled {
		logrus.Info("Metrics disabled")
		return
	}
	rtr := http.NewServeMux()
	rtr.HandleFunc("/metrics", internalHandler)
	rtr.HandleFunc("/_media/metrics", internalHandler)

	address := config.Get().Metrics.BindAddress + ":" + strconv.Itoa(config.Get().Metrics.Port)
	srv = &http.Server{Addr: address, Handler: rtr}
	go func() {
		logrus.WithField("address", address).Info("Started metrics listener. Listening at http://" + address)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			logrus.Fatal(err)
		}
	}()
}

func Reload() {
	Stop()
	Init()
}

func Stop() {
	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			panic(err)
		}
	}
}
