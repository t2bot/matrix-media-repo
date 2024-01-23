package api

import (
	"net/http"
	"strings"

	"github.com/t2bot/matrix-media-repo/api/_routers"
)

type branch struct {
	string
	http.Handler
}

type splitBranch struct {
	segments []string
	handler  http.Handler
}

func branchedRoute(branches []branch) http.Handler {
	sbranches := make([]splitBranch, len(branches))
	for i, b := range branches {
		sbranches[i] = splitBranch{
			segments: strings.Split(b.string, "/"),
			handler:  b.Handler,
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		catchAll := _routers.GetParam("branch", r)
		if catchAll[0] == '/' {
			catchAll = catchAll[1:]
		}
		params := strings.Split(catchAll, "/")
		for _, b := range sbranches {
			if b.segments[0][0] == ':' || b.segments[0] == params[0] {
				if len(b.segments) != len(params) {
					continue
				}
				for i, s := range b.segments {
					if s[0] == ':' {
						r = _routers.ForceSetParam(s[1:], params[i], r)
					}
				}
				b.handler.ServeHTTP(w, r)
				return
			}
		}
		notFoundFn(w, r)
	})
}
