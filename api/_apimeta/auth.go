package _apimeta

import (
	"net/http"

	"github.com/getsentry/sentry-go"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/matrix"
	"github.com/turt2live/matrix-media-repo/util"
)

type UserInfo struct {
	UserId      string
	AccessToken string
	IsShared    bool
}

type ServerInfo struct {
	ServerName string
}

func GetRequestUserAdminStatus(r *http.Request, rctx rcontext.RequestContext, user UserInfo) (bool, bool) {
	isGlobalAdmin := util.IsGlobalAdmin(user.UserId) || user.IsShared
	isLocalAdmin, err := matrix.IsUserAdmin(rctx, r.Host, user.AccessToken, r.RemoteAddr)
	if err != nil {
		sentry.CaptureException(err)
		rctx.Log.Debug("Error verifying local admin: ", err)
		return isGlobalAdmin, false
	}

	return isGlobalAdmin, isLocalAdmin
}
