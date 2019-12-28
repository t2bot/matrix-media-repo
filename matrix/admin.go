package matrix

import (
	"time"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

func IsUserAdmin(ctx rcontext.RequestContext, serverName string, accessToken string, ipAddr string) (bool, error) {
	fakeUser := "@media.repo.admin.check:" + serverName
	hs, cb := getBreakerAndConfig(serverName)

	isAdmin := false
	var replyError error
	replyError = cb.CallContext(ctx, func() error {

		response := &whoisResponse{}
		path := "/_matrix/client/unstable/admin/whois/"
		if hs.AdminApiKind == "synapse" {
			path = "/_synapse/admin/v1/whois/"
		}
		url := makeUrl(hs.ClientServerApi, path, fakeUser)
		err := doRequest(ctx, "GET", url, nil, response, accessToken, ipAddr)
		if err != nil {
			err, replyError = filterError(err)
			return err
		}

		isAdmin = true // if we made it this far, that is
		return nil
	}, 1*time.Minute)

	return isAdmin, replyError
}

func ListMedia(ctx rcontext.RequestContext, serverName string, accessToken string, roomId string, ipAddr string) (*mediaListResponse, error) {
	hs, cb := getBreakerAndConfig(serverName)

	response := &mediaListResponse{}
	var replyError error
	replyError = cb.CallContext(ctx, func() error {
		path := "/_matrix/client/unstable/admin/room/"
		if hs.AdminApiKind == "synapse" {
			path = "/_synapse/admin/v1/room/"
		}
		url := makeUrl(hs.ClientServerApi, path, roomId, "/media")
		err := doRequest(ctx, "GET", url, nil, response, accessToken, ipAddr)
		if err != nil {
			err, replyError = filterError(err)
			return err
		}

		return nil
	}, 5*time.Minute) // longer timeout because this may take a while

	return response, replyError
}
