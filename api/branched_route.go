package api

import (
	"net/http"
	"strings"

	"github.com/t2bot/matrix-media-repo/api/routers"
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
	for i, branch := range branches {
		sbranches[i] = splitBranch{
			segments: strings.Split(branch.string, "/"),
			handler:  branch.Handler,
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		catchAll := routers.GetParam("branch", r)
		if catchAll[0] == '/' {
			catchAll = catchAll[1:]
		}
		params := strings.Split(catchAll, "/")
		for _, branch := range sbranches {
			if branch.segments[0][0] == ':' || branch.segments[0] == params[0] {
				if len(branch.segments) != len(params) {
					continue
				}
				for i, segment := range branch.segments {
					if segment[0] == ':' {
						r = routers.ForceSetParam(segment[1:], params[i], r)
					}
				}
				branch.handler.ServeHTTP(w, r)
				return
			}
		}
		notFoundFn(w, r)
	})
}
