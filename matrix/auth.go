package matrix

import (
	"net/url"
	"time"

	"github.com/pkg/errors"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

var ErrInvalidToken = errors.New("Missing or invalid access token")

func GetUserIdFromToken(ctx rcontext.RequestContext, serverName string, accessToken string, appserviceUserId string, ipAddr string) (string, error) {
	if accessToken == "" {
		return "", ErrInvalidToken
	}

	hs, cb := getBreakerAndConfig(serverName)

	userId := ""
	var replyError error
	var authError error
	replyError = cb.CallContext(ctx, func() error {
		query := map[string]string{}
		if appserviceUserId != "" {
			query["user_id"] = appserviceUserId
		}

		response := &userIdResponse{}
		target, _ := url.Parse(makeUrl(hs.ClientServerApi, "/_matrix/client/r0/account/whoami"))
		q := target.Query()
		for k, v := range query {
			q.Set(k, v)
		}
		target.RawQuery = q.Encode()
		err := doRequest(ctx, "GET", target.String(), nil, response, accessToken, ipAddr)
		if err != nil {
			ctx.Log.Warn("Error from homeserver: ", err)
			err, authError = filterError(err)
			return err
		}

		userId = response.UserId
		return nil
	}, 1*time.Minute)

	if authError != nil {
		return userId, authError
	}
	return userId, replyError
}
