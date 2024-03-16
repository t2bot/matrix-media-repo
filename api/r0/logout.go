package r0

import (
	"errors"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/api/apimeta"
	"github.com/t2bot/matrix-media-repo/api/responses"

	"github.com/t2bot/matrix-media-repo/api/auth_cache"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

func Logout(r *http.Request, rctx rcontext.RequestContext, user apimeta.UserInfo) interface{} {
	err := auth_cache.InvalidateToken(rctx, user.AccessToken, user.UserId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return responses.InternalServerError(errors.New("unable to logout"))
	}
	return responses.EmptyResponse{}
}

func LogoutAll(r *http.Request, rctx rcontext.RequestContext, user apimeta.UserInfo) interface{} {
	err := auth_cache.InvalidateAllTokens(rctx, user.AccessToken, user.UserId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return responses.InternalServerError(errors.New("unable to logout"))
	}
	return responses.EmptyResponse{}
}
