package r0

import (
	"errors"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/api/_responses"

	"github.com/t2bot/matrix-media-repo/api/_auth_cache"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

func Logout(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	err := _auth_cache.InvalidateToken(rctx, user.AccessToken, user.UserId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(errors.New("unable to logout"))
	}
	return _responses.EmptyResponse{}
}

func LogoutAll(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	err := _auth_cache.InvalidateAllTokens(rctx, user.AccessToken, user.UserId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(errors.New("unable to logout"))
	}
	return _responses.EmptyResponse{}
}
