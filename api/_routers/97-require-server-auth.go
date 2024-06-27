package _routers

import (
	"errors"
	"net/http"

	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/matrix"
)

type GeneratorWithServerFn = func(r *http.Request, ctx rcontext.RequestContext, server _apimeta.ServerInfo) interface{}

func RequireServerAuth(generator GeneratorWithServerFn) GeneratorFn {
	return func(r *http.Request, ctx rcontext.RequestContext) interface{} {
		serverName, err := matrix.ValidateXMatrixAuth(r, true)
		if err != nil {
			ctx.Log.Debug("Error with X-Matrix auth: ", err)
			if errors.Is(err, matrix.ErrNoXMatrixAuth) {
				return &_responses.ErrorResponse{
					Code:         common.ErrCodeUnauthorized,
					Message:      "no auth provided (required)",
					InternalCode: common.ErrCodeMissingToken,
				}
			}
			if errors.Is(err, matrix.ErrWrongDestination) {
				return &_responses.ErrorResponse{
					Code:         common.ErrCodeUnauthorized,
					Message:      "no auth provided for this destination (required)",
					InternalCode: common.ErrCodeBadRequest,
				}
			}
			return &_responses.ErrorResponse{
				Code:         common.ErrCodeForbidden,
				Message:      "invalid auth provided (required)",
				InternalCode: common.ErrCodeBadRequest,
			}
		}
		return generator(r, ctx, _apimeta.ServerInfo{
			ServerName: serverName,
		})
	}
}
