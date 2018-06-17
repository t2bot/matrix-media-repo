package matrix

import (
	"context"
	"net/url"
	"time"

	"github.com/pkg/errors"
)

var ErrNoToken = errors.New("Missing access token")

func GetUserIdFromToken(ctx context.Context, serverName string, accessToken string, appserviceUserId string) (string, error) {
	if accessToken == "" {
		return "", ErrNoToken
	}

	hs, cb := getBreakerAndConfig(serverName)

	userId := ""
	var replyError error
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
		err := doRequest("GET", target.String(), nil, response, accessToken)
		if err != nil {
			err, replyError = filterError(err)
			return err
		}

		userId = response.UserId
		return nil
	}, 1*time.Minute)

	if replyError == nil {
		return userId, nil
	}
	return userId, replyError
}
