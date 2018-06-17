package matrix

import (
	"context"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/types"
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

func GetUserIdFromBearerToken(ctx context.Context, bearerToken *types.BearerToken, contentToken string) (string, error) {
	// HACK: This is a workaround for the federation problem not being solved.
	// See also: MSC701 / #103
	if bearerToken.EncryptedToken == contentToken {
		logrus.Warn("Server token used for the bearer token payload - accepting as-is")
		return "REMOTE_SERVER", nil
	}

	accessToken, err := bearerToken.RetrieveAccessToken(contentToken)
	if err != nil {
		logrus.Warn("Failed to get access token from bearer token: ", err.Error())
		return "", common.ErrFailedAuthCheck
	}

	userId, err := GetUserIdFromToken(ctx, bearerToken.Host, accessToken, bearerToken.AppserviceUserId)
	if err != nil {
		logrus.Warn("Failed to verify access token: ", err.Error())
		return "", common.ErrFailedAuthCheck
	}

	return userId, nil
}