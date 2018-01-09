package util

import (
	"context"

	"github.com/matrix-org/gomatrix"
)

type userIdResponse struct {
	UserId string `json:"user_id"`
}

func GetUserIdFromToken(ctx context.Context, serverName string, accessToken string) (string, error) {
	hs := GetHomeserverConfig(serverName)
	mtxClient, err := gomatrix.NewClient(hs.ClientServerApi, "", accessToken)
	if err != nil {
		return "", err
	}

	response := &userIdResponse{}
	url := mtxClient.BuildURL("/account/whoami")
	_, err = mtxClient.MakeRequest("GET", url, nil, response)
	if err != nil {
		return "", err
	}

	return response.UserId, nil
}
