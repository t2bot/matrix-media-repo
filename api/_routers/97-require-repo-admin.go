package _routers

import (
	"errors"
	"net/http"

	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/util"
)

func RequireRepoAdmin(generator GeneratorWithUserFn) GeneratorFn {
	return func(r *http.Request, ctx rcontext.RequestContext) interface{} {
		return RequireAccessToken(func(r *http.Request, ctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
			if user.UserId == "" {
				panic(errors.New("safety check failed: Repo admin access check received empty user ID"))
			}

			if !util.IsGlobalAdmin(user.UserId) {
				return _responses.AuthFailed()
			}

			return generator(r, ctx, user)
		})
	}
}
