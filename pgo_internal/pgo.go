package pgo_internal

import (
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/pgo-fleet/embedded"
)

func init() {
	pgo.ErrorFunc = func(err error) {
		sentry.CaptureException(err)
	}
}

func Enable(submitUrl string, submitKey string) {
	endpoint, err := pgo.NewCollectorEndpoint(submitUrl, submitKey)
	if err != nil {
		panic(err)
	}

	pgo.Enable(1*time.Hour, 5*time.Minute, endpoint)
}

func Disable() {
	pgo.Disable()
}
