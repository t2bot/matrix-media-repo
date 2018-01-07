package client

import (
	"net/http"

	"github.com/turt2live/matrix-media-repo/rcontext"
)

func NotFoundHandler(w http.ResponseWriter, r *http.Request, i rcontext.RequestInfo) interface{} {
	return NotFoundError()
}

func MethodNotAllowedHandler(w http.ResponseWriter, r *http.Request, i rcontext.RequestInfo) interface{} {
	return MethodNotAllowed()
}
