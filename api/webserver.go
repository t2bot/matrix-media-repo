package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/didip/tollbooth/v7"
	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/limits"
)

var srv *http.Server
var waitGroup = &sync.WaitGroup{}
var reload = false

func Init() *sync.WaitGroup {
	address := net.JoinHostPort(config.Get().General.BindAddress, strconv.Itoa(config.Get().General.Port))

	//defer func() {
	//	if err := recover(); err != nil {
	//		logrus.Fatal(err)
	//	}
	//}()

	handler := buildRoutes()

	if config.Get().RateLimit.Enabled {
		logrus.Debug("Enabling rate limit")
		handler = tollbooth.LimitHandler(limits.GetRequestLimiter(), handler)
	}

	// Note: we bind Sentry here to ensure we capture *everything*
	sentryHandler := sentryhttp.New(sentryhttp.Options{})
	srv = &http.Server{Addr: address, Handler: sentryHandler.Handle(handler)}
	reload = false

	go func() {
		//goland:noinspection HttpUrlsUsage
		logrus.WithField("address", address).Info("Started up. Listening at http://" + address)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			sentry.CaptureException(err)
			logrus.Fatal(err)
		}

		// Only notify the main thread that we're done if we're actually done
		srv = nil
		if !reload {
			waitGroup.Done()
		}
	}()

	return waitGroup
}

func Reload() {
	reload = true

	// Stop the server first
	Stop()

	// Reload the web server, ignoring the wait group (because we don't care to wait here)
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
