package matrix

import (
	"context"
	"time"

	"github.com/matrix-org/gomatrix"
)

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
