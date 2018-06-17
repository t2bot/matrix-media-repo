package matrix

import (
	"context"
	"time"
)

func IsUserAdmin(ctx context.Context, serverName string, accessToken string) (bool, error) {
	fakeUser := "@media.repo.admin.check:" + serverName
	hs, cb := getBreakerAndConfig(serverName)

	isAdmin := false
	var replyError error
	replyError = cb.CallContext(ctx, func() error {

		response := &whoisResponse{}
		url := makeUrl(hs.ClientServerApi, "/_matrix/client/r0/admin/whois/", fakeUser)
		err := doRequest("GET", url, nil, response, accessToken)
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
		url := makeUrl(hs.ClientServerApi, "/_matrix/client/r0/admin/room/", roomId, "/media")
		err := doRequest("GET", url, nil, response, accessToken)
		if err != nil {
			err, replyError = filterError(err)
			return err
		}

		return nil
	}, 5*time.Minute) // longer timeout because this may take a while

	return response, replyError
}
