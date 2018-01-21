package matrix

import (
	"context"
	"time"

	"github.com/matrix-org/gomatrix"
)

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

func ListMedia(ctx context.Context, serverName string, accessToken string, roomId string) (*mediaListResponse, error) {
	hs, cb := getBreakerAndConfig(serverName)

	response := &mediaListResponse{}
	var replyError error
	cb.CallContext(ctx, func() error {
		mtxClient, err := gomatrix.NewClient(hs.ClientServerApi, "", accessToken)
		if err != nil {
			return filterError(err, &replyError)
		}

		url := mtxClient.BuildURL("/admin/room/", roomId, "/media")
		_, err = mtxClient.MakeRequest("GET", url, nil, response)
		if err != nil {
			return filterError(err, &replyError)
		}

		return nil
	}, 5*time.Minute) // longer timeout because this may take a while

	return response, replyError
}
