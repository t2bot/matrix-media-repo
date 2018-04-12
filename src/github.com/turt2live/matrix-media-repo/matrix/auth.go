package matrix

import (
	"context"
	"time"

	"github.com/matrix-org/gomatrix"
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
	cb.CallContext(ctx, func() error {
		mtxClient, err := gomatrix.NewClient(hs.ClientServerApi, "", accessToken)
		if err != nil {
			err, replyError = filterError(err)
			return err
		}

		query := map[string]string{}
		if appserviceUserId != "" {
			query["user_id"] = appserviceUserId
		}

		response := &userIdResponse{}
		url := mtxClient.BuildURLWithQuery([]string{"/account/whoami"}, query)
		_, err = mtxClient.MakeRequest("GET", url, nil, response)
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
