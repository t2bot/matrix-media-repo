package _routers

import (
	"net/http"

	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/matrix"
)

type GeneratorWithServerFn = func(r *http.Request, ctx rcontext.RequestContext, server _apimeta.ServerInfo) interface{}

func RequireServerAuth(generator GeneratorWithServerFn) GeneratorFn {
	return func(r *http.Request, ctx rcontext.RequestContext) interface{} {
		serverName, err := matrix.ValidateXMatrixAuth(r, true)
		if err != nil {
			ctx.Log.Debug("Error with X-Matrix auth: ", err)
			return &_responses.ErrorResponse{
				Code:         common.ErrCodeForbidden,
				Message:      "no auth provided (required)",
				InternalCode: common.ErrCodeMissingToken,
			}
		}
		return generator(r, ctx, _apimeta.ServerInfo{
			ServerName: serverName,
		})
	}
}
