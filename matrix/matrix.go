package matrix

import (
	"errors"
	"sync"

	"github.com/rubyist/circuitbreaker"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/config"
)

var breakers = &sync.Map{}

func getBreakerAndConfig(serverName string) (*config.DomainRepoConfig, *circuit.Breaker) {
	hs := config.GetDomain(serverName)

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
	var httpErr *errorResponse
	if errors.As(err, &httpErr) {
		// We send back our own version of errors to ensure we can filter them out elsewhere
		if httpErr.ErrorCode == common.ErrCodeUnknownToken {
			return nil, ErrInvalidToken
		} else if httpErr.ErrorCode == common.ErrCodeNoGuests {
			return nil, ErrGuestToken
		}
	}

	return err, err
}
