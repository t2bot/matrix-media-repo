package metrics

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
)

func Init() {
	if !config.Get().Metrics.Enabled {
		logrus.Info("Metrics disabled")
		return
	}
	rtr := http.NewServeMux()
	rtr.Handle("/metrics", promhttp.Handler())
	rtr.Handle("/_media/metrics", promhttp.Handler())
	go func() {
		address := config.Get().Metrics.BindAddress + ":" + strconv.Itoa(config.Get().Metrics.Port)
		logrus.WithField("address", address).Info("Started metrics listener. Listening at http://" + address)
		logrus.Fatal(http.ListenAndServe(address, rtr))
	}()
}
