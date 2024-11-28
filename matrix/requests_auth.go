package matrix

import (
	"errors"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

var ErrInvalidToken = errors.New("missing or invalid access token")
var ErrGuestToken = errors.New("token belongs to a guest")

func GetUserIdFromToken(ctx rcontext.RequestContext, serverName string, accessToken string, appserviceUserId string, ipAddr string) (string, bool, error) {
	response := &userIdResponse{}
	err := doBreakerRequest(ctx, serverName, accessToken, appserviceUserId, ipAddr, "GET", "/_matrix/client/v3/account/whoami", response)
	if err != nil {
		return "", false, err
	}
	return response.UserId, response.IsGuest || response.IsGuest2, nil
}

func Logout(ctx rcontext.RequestContext, serverName string, accessToken string, appserviceUserId string, ipAddr string) error {
	response := &emptyResponse{}
	err := doBreakerRequest(ctx, serverName, accessToken, appserviceUserId, ipAddr, "POST", "/_matrix/client/v3/logout", response)
	if err != nil {
		return err
	}
	return nil
}

func LogoutAll(ctx rcontext.RequestContext, serverName string, accessToken string, appserviceUserId string, ipAddr string) error {
	response := &emptyResponse{}
	err := doBreakerRequest(ctx, serverName, accessToken, appserviceUserId, ipAddr, "POST", "/_matrix/client/v3/logout/all", response)
	if err != nil {
		return err
	}
	return nil
}
