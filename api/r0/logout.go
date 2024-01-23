package r0

import (
	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/api/_responses"

	"net/http"

	"github.com/t2bot/matrix-media-repo/api/_auth_cache"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

func Logout(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	err := _auth_cache.InvalidateToken(rctx, user.AccessToken, user.UserId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("unable to logout")
	}
	return _responses.EmptyResponse{}
}

func LogoutAll(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	err := _auth_cache.InvalidateAllTokens(rctx, user.AccessToken, user.UserId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("unable to logout")
	}
	return _responses.EmptyResponse{}
}
