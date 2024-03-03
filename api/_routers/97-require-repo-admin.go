package _routers

import (
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/api/apimeta"
	"github.com/t2bot/matrix-media-repo/api/responses"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/util"
)

func RequireRepoAdmin(generator GeneratorWithUserFn) GeneratorFn {
	return func(r *http.Request, ctx rcontext.RequestContext) interface{} {
		return RequireAccessToken(func(r *http.Request, ctx rcontext.RequestContext, user apimeta.UserInfo) interface{} {
			if user.UserId == "" {
				logrus.Error("safety check failed: Repo admin access check received empty user ID")
				return responses.AuthFailed()
			}

			if !user.IsShared && !util.IsGlobalAdmin(user.UserId) {
				return responses.AuthFailed()
			}

			return generator(r, ctx, user)
		})(r, ctx)
	}
}
