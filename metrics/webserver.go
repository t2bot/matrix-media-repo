package metrics

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common/config"
)

var srv *http.Server

func internalHandler(res http.ResponseWriter, req *http.Request) {
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

	address := net.JoinHostPort(config.Get().Metrics.BindAddress, strconv.Itoa(config.Get().Metrics.Port))
	srv = &http.Server{Addr: address, Handler: rtr}
	go func() {
		//goland:noinspection HttpUrlsUsage
		logrus.WithField("address", address).Info("Started metrics listener. Listening at http://" + address)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			sentry.CaptureException(err)
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
