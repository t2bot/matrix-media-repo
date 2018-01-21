package matrix

import (
	"github.com/matrix-org/gomatrix"
	"github.com/rubyist/circuitbreaker"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/util"
)

var breakers = make(map[string]*circuit.Breaker)

func getBreakerAndConfig(serverName string) (*config.HomeserverConfig, *circuit.Breaker) {
	hs := util.GetHomeserverConfig(serverName)

	cb, hasCb := breakers[hs.Name]
	if !hasCb {
		backoffAt := int64(hs.BackoffAt)
		if backoffAt <= 0 {
			backoffAt = 10 // default to 10 for those who don't have this set
		}
		cb = circuit.NewConsecutiveBreaker(backoffAt)
		breakers[hs.Name] = cb
	}

	return hs, cb
}

func filterError(err error, replyError *error) error {
	if err == nil {
		replyError = nil
		return nil
	}

	// Unknown token errors should be filtered out explicitly to ensure we don't break on bad requests
	if httpErr, ok := err.(gomatrix.HTTPError); ok {
		if respErr, ok := httpErr.WrappedError.(gomatrix.RespError); ok {
			if respErr.ErrCode == "M_UNKNOWN_TOKEN" {
				replyError = &err // we still want to send the error to the caller though
				return nil
			}
		}
	}

	replyError = &err
	return err
}
