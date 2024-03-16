package routers

import (
	"errors"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/api/apimeta"
	"github.com/t2bot/matrix-media-repo/api/auth_cache"
	"github.com/t2bot/matrix-media-repo/api/responses"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/matrix"
	"github.com/t2bot/matrix-media-repo/util"
)

type GeneratorWithUserFn = func(r *http.Request, ctx rcontext.RequestContext, user apimeta.UserInfo) interface{}

func RequireAccessToken(generator GeneratorWithUserFn) GeneratorFn {
	return func(r *http.Request, ctx rcontext.RequestContext) interface{} {
		accessToken := util.GetAccessTokenFromRequest(r)
		if accessToken == "" {
			return &responses.ErrorResponse{
				Code:         common.ErrCodeMissingToken,
				Message:      "no token provided (required)",
				InternalCode: common.ErrCodeMissingToken,
			}
		}
		if config.Get().SharedSecret.Enabled && accessToken == config.Get().SharedSecret.Token {
			ctx = ctx.LogWithFields(logrus.Fields{"sharedSecretAuth": true})
			return generator(r, ctx, apimeta.UserInfo{
				UserId:      "@sharedsecret",
				AccessToken: accessToken,
				IsShared:    true,
			})
		}
		appserviceUserId := util.GetAppserviceUserIdFromRequest(r)
		userId, err := auth_cache.GetUserId(ctx, accessToken, appserviceUserId)
		if err != nil || userId == "" {
			if errors.Is(err, matrix.ErrGuestToken) {
				return responses.GuestAuthFailed()
			}
			if err != nil && !errors.Is(err, matrix.ErrInvalidToken) {
				sentry.CaptureException(err)
				ctx.Log.Error("Error verifying token: ", err)
				return responses.InternalServerError(errors.New("unexpected error validating access token"))
			}
			return responses.AuthFailed()
		}

		ctx = ctx.LogWithFields(logrus.Fields{"authUserId": userId})
		return generator(r, ctx, apimeta.UserInfo{
			UserId:      userId,
			AccessToken: accessToken,
			IsShared:    false,
		})
	}
}
