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
	replyError = cb.CallContext(ctx, func() error {
		mtxClient, err := gomatrix.NewClient(hs.ClientServerApi, "", accessToken)
		if err != nil {
			err, replyError = filterError(err)
			return err
		}

		response := &whoisResponse{}
		url := mtxClient.BuildURL("/admin/whois/", fakeUser)
		_, err = mtxClient.MakeRequest("GET", url, nil, response)
		if err != nil {
			err, replyError = filterError(err)
			return err
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
	replyError = cb.CallContext(ctx, func() error {
		mtxClient, err := gomatrix.NewClient(hs.ClientServerApi, "", accessToken)
		if err != nil {
			err, replyError = filterError(err)
			return err
		}

		url := mtxClient.BuildURL("/admin/room/", roomId, "/media")
		_, err = mtxClient.MakeRequest("GET", url, nil, response)
		if err != nil {
			err, replyError = filterError(err)
			return err
		}

		return nil
	}, 5*time.Minute) // longer timeout because this may take a while

	return response, replyError
}
