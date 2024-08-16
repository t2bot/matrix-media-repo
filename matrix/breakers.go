package matrix

import (
	"net/url"
	"sync"
	"time"

	"github.com/rubyist/circuitbreaker"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/util"
)

var breakers = &sync.Map{}
var federationBreakers = &sync.Map{}

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

func getFederationBreaker(hostname string) *circuit.Breaker {
	var cb *circuit.Breaker
	cbRaw, hasCb := federationBreakers.Load(hostname)
	if !hasCb {
		backoffAt := int64(config.Get().Federation.BackoffAt)
		if backoffAt <= 0 {
			backoffAt = 20 // default to 20 for those who don't have this set
		}
		cb = circuit.NewConsecutiveBreaker(backoffAt)
		federationBreakers.Store(hostname, cb)
	} else {
		cb = cbRaw.(*circuit.Breaker)
	}
	return cb
}

func doBreakerRequest(ctx rcontext.RequestContext, serverName string, accessToken string, appserviceUserId string, ipAddr string, method string, path string, resp interface{}) error {
	hs, cb := getBreakerAndConfig(serverName)

	var replyError error
	var authError error
	replyError = cb.CallContext(ctx, func() error {
		query := map[string]string{}
		if appserviceUserId != "" {
			query["user_id"] = appserviceUserId
		}

		target, _ := url.Parse(util.MakeUrl(hs.ClientServerApi, path))
		q := target.Query()
		for k, v := range query {
			q.Set(k, v)
		}
		target.RawQuery = q.Encode()
		err := doRequest(ctx, method, target.String(), nil, resp, accessToken, ipAddr)
		if err != nil {
			ctx.Log.Debug("Error from homeserver: ", err)
			err, authError = filterError(err)
			return err
		}
		return nil
	}, 1*time.Minute)

	if authError != nil {
		return authError
	}
	return replyError
}
