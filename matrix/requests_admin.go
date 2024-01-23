package matrix

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/util"
)

func IsUserAdmin(ctx rcontext.RequestContext, serverName string, accessToken string, ipAddr string) (bool, error) {
	fakeUser := "@media.repo.admin.check:" + serverName
	hs, cb := getBreakerAndConfig(serverName)

	isAdmin := false
	var replyError error
	replyError = cb.CallContext(ctx, func() error {
		response := &whoisResponse{}
		// the whois endpoint is part of the spec, meaning we can avoid per-homeserver support
		path := fmt.Sprintf("/_matrix/client/v3/admin/whois/%s", url.PathEscape(fakeUser))
		if hs.AdminApiKind == "synapse" { // synapse is special, dendrite is not
			path = fmt.Sprintf("/_synapse/admin/v1/whois/%s", url.PathEscape(fakeUser))
		}
		urlStr := util.MakeUrl(hs.ClientServerApi, path)
		err := doRequest(ctx, "GET", urlStr, nil, response, accessToken, ipAddr)
		if err != nil {
			err, replyError = filterError(err)
			return err
		}

		isAdmin = true // if we made it this far, that is
		return nil
	}, 1*time.Minute)

	return isAdmin, replyError
}

func ListMedia(ctx rcontext.RequestContext, serverName string, accessToken string, roomId string, ipAddr string) (*MediaListResponse, error) {
	hs, cb := getBreakerAndConfig(serverName)

	response := &MediaListResponse{}
	var replyError error
	replyError = cb.CallContext(ctx, func() error {
		path := ""
		if hs.AdminApiKind == "synapse" {
			path = fmt.Sprintf("/_synapse/admin/v1/room/%s/media", url.PathEscape(roomId))
		}
		if hs.AdminApiKind == "dendrite" {
			return errors.New("this function is not supported when backed by Dendrite")
		}
		if path == "" {
			return errors.New("unable to query media for homeserver: wrong or incompatible adminApiKind")
		}
		urlStr := util.MakeUrl(hs.ClientServerApi, path)
		err := doRequest(ctx, "GET", urlStr, nil, response, accessToken, ipAddr)
		if err != nil {
			err, replyError = filterError(err)
			return err
		}

		return nil
	}, 5*time.Minute) // longer timeout because this may take a while

	return response, replyError
}
