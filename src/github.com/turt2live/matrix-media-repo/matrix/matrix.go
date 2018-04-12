package matrix

import (
	"sync"

	"github.com/matrix-org/gomatrix"
	"github.com/rubyist/circuitbreaker"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/util"
)

var breakers = &sync.Map{}

func getBreakerAndConfig(serverName string) (*config.HomeserverConfig, *circuit.Breaker) {
	hs := util.GetHomeserverConfig(serverName)

	var cb *circuit.Breaker
	cbRaw, hasCb := breakers.Load(hs.Name)
	if !hasCb {
		backoffAt := int64(hs.BackoffAt)
		if backoffAt <= 0 {
			backoffAt = 10 // default to 10 for those who don't have this set
		}
		cb = circuit.NewConsecutiveBreaker(backoffAt)
		breakers.Store(hs.Name, cb)
	} else {
		cb = cbRaw.(*circuit.Breaker)
	}

	return hs, cb
}

func filterError(err error) (error, error) {
	if err == nil {
		return nil, nil
	}

	// Unknown token errors should be filtered out explicitly to ensure we don't break on bad requests
	if httpErr, ok := err.(gomatrix.HTTPError); ok {
		if respErr, ok := httpErr.WrappedError.(gomatrix.RespError); ok {
			if respErr.ErrCode == "M_UNKNOWN_TOKEN" {
				// We send back our own version of UNKNOWN_TOKEN to ensure we can filter it out elsewhere
				return nil, ErrNoToken
			}
		}
	}

	return err, err
}
