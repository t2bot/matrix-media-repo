package matrix

import (
	"net/url"
	"time"

	"github.com/pkg/errors"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/util"
)

var ErrInvalidToken = errors.New("Missing or invalid access token")
var ErrGuestToken = errors.New("Token belongs to a guest")

func doBreakerRequest(ctx rcontext.RequestContext, serverName string, accessToken string, appserviceUserId string, ipAddr string, method string, path string, resp interface{}) error {
	if accessToken == "" {
		return ErrInvalidToken
	}

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
			ctx.Log.Warn("Error from homeserver: ", err)
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

func GetUserIdFromToken(ctx rcontext.RequestContext, serverName string, accessToken string, appserviceUserId string, ipAddr string) (string, error) {
	response := &userIdResponse{}
	err := doBreakerRequest(ctx, serverName, accessToken, appserviceUserId, ipAddr, "GET", "/_matrix/client/r0/account/whoami", response)
	if err != nil {
		return "", err
	}
	if response.IsGuest || response.IsGuest2 {
		return "", ErrGuestToken
	}
	return response.UserId, nil
}

func Logout(ctx rcontext.RequestContext, serverName string, accessToken string, appserviceUserId string, ipAddr string) error {
	response := &emptyResponse{}
	err := doBreakerRequest(ctx, serverName, accessToken, appserviceUserId, ipAddr, "POST", "/_matrix/client/r0/logout", response)
	if err != nil {
		return err
	}
	return nil
}

func LogoutAll(ctx rcontext.RequestContext, serverName string, accessToken string, appserviceUserId string, ipAddr string) error {
	response := &emptyResponse{}
	err := doBreakerRequest(ctx, serverName, accessToken, appserviceUserId, ipAddr, "POST", "/_matrix/client/r0/logout/all", response)
	if err != nil {
		return err
	}
	return nil
}
