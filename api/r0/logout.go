package r0

import (
	"net/http"

	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/api/auth_cache"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

func Logout(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	err := auth_cache.InvalidateToken(rctx, user.AccessToken, user.UserId)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("unable to logout")
	}
	return api.EmptyResponse{}
}

func LogoutAll(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	err := auth_cache.InvalidateAllTokens(rctx, user.AccessToken, user.UserId)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("unable to logout")
	}
	return api.EmptyResponse{}
}
