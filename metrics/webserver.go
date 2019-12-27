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

func Init() {
	if !config.Get().Metrics.Enabled {
		logrus.Info("Metrics disabled")
		return
	}
	rtr := http.NewServeMux()
	rtr.Handle("/metrics", promhttp.Handler())
	rtr.Handle("/_media/metrics", promhttp.Handler())

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
