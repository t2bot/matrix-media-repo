package matrix

import (
	"context"
	"time"

	"github.com/matrix-org/gomatrix"
	"github.com/rubyist/circuitbreaker"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/util"
)

type userIdResponse struct {
	UserId string `json:"user_id"`
}
type whoisResponse struct {
	// We don't actually care about any of the fields here
}

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

func IsUserAdmin(ctx context.Context, serverName string, accessToken string) (bool, error) {
	fakeUser := "@media.repo.admin.check:" + serverName
	hs, cb := getBreakerAndConfig(serverName)

	isAdmin := false
	var replyError error
	cb.CallContext(ctx, func() error {
		mtxClient, err := gomatrix.NewClient(hs.ClientServerApi, "", accessToken)
		if err != nil {
			return filterError(err, &replyError)
		}

		response := &whoisResponse{}
		url := mtxClient.BuildURL("/admin/whois/", fakeUser)
		_, err = mtxClient.MakeRequest("GET", url, nil, response)
		if err != nil {
			return filterError(err, &replyError)
		}

		isAdmin = true // if we made it this far, that is
		return nil
	}, 1*time.Minute)

	return isAdmin, replyError
}

func GetUserIdFromToken(ctx context.Context, serverName string, accessToken string) (string, error) {
	hs, cb := getBreakerAndConfig(serverName)

	userId := ""
	var replyError error
	cb.CallContext(ctx, func() error {
		mtxClient, err := gomatrix.NewClient(hs.ClientServerApi, "", accessToken)
		if err != nil {
			return filterError(err, &replyError)
		}

		response := &userIdResponse{}
		url := mtxClient.BuildURL("/account/whoami")
		_, err = mtxClient.MakeRequest("GET", url, nil, response)
		if err != nil {
			return filterError(err, &replyError)
		}

		userId = response.UserId
		return nil
	}, 1*time.Minute)

	return userId, replyError
}
