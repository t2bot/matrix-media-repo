package api

import (
	"context"
	"encoding/json"
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
	"github.com/t2bot/matrix-media-repo/api/responses"
	"github.com/t2bot/matrix-media-repo/common/config"
)

var (
	srv       *http.Server
	waitGroup = &sync.WaitGroup{}
	reload    = false
)

func Init() *sync.WaitGroup {
	address := net.JoinHostPort(config.Get().General.BindAddress, strconv.Itoa(config.Get().General.Port))

	handler := buildRoutes()

	if config.Get().RateLimit.Enabled {
		logrus.Debug("Enabling rate limit")
		limiter := tollbooth.NewLimiter(0, nil)
		limiter.SetIPLookups([]string{"X-Forwarded-For", "X-Real-IP", "RemoteAddr"})
		limiter.SetTokenBucketExpirationTTL(time.Hour)
		limiter.SetBurst(config.Get().RateLimit.BurstCount)
		limiter.SetMax(config.Get().RateLimit.RequestsPerSecond)

		reponse, _ := json.Marshal(responses.RateLimitReached())
		limiter.SetMessage(string(reponse))
		limiter.SetMessageContentType("application/json")

		handler = tollbooth.LimitHandler(limiter, handler)
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
			logrus.Fatalf("Could not gracefully shutdown the server: %v", err)
		}
	}
}
