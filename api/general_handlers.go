package api

import (
	"net/http"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

func NotFoundHandler(r *http.Request, rctx rcontext.RequestContext) interface{} {
	return NotFoundError()
}

func MethodNotAllowedHandler(r *http.Request, rctx rcontext.RequestContext) interface{} {
	return MethodNotAllowed()
}

func EmptyResponseHandler(r *http.Request, rctx rcontext.RequestContext) interface{} {
	return &EmptyResponse{}
}
