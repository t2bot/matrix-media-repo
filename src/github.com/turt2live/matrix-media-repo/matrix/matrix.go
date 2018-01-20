package matrix

import (
	"context"
	"time"

	"github.com/matrix-org/gomatrix"
	"github.com/rubyist/circuitbreaker"
	"github.com/turt2live/matrix-media-repo/util"
)

type userIdResponse struct {
	UserId string `json:"user_id"`
}

var breakers = make(map[string]*circuit.Breaker)

func GetUserIdFromToken(ctx context.Context, serverName string, accessToken string) (string, error) {
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

	userId := ""
	var replyError error
	err := cb.CallContext(ctx, func() error {
		mtxClient, err := gomatrix.NewClient(hs.ClientServerApi, "", accessToken)
		if err != nil {
			return err
		}

		response := &userIdResponse{}
		url := mtxClient.BuildURL("/account/whoami")
		_, err = mtxClient.MakeRequest("GET", url, nil, response)
		if err != nil {
			// Unknown token errors should be filtered out explicitly to ensure we don't break on bad requests
			if httpErr, ok := err.(gomatrix.HTTPError); ok {
				if respErr, ok := httpErr.WrappedError.(gomatrix.RespError); ok {
					if respErr.ErrCode == "M_UNKNOWN_TOKEN" {
						replyError = err // we still want to send the error to the caller though
						return nil
					}
				}
			}
			return err
		}

		userId = response.UserId
		return nil
	}, 1*time.Minute)

	if replyError != nil {
		err = replyError
	}

	return userId, err
}
