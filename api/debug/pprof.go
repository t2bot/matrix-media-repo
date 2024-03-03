package debug

import (
	"encoding/json"
	"net/http"
	"net/http/pprof"

	"github.com/julienschmidt/httprouter"
)

func BindPprofEndpoints(httpMux *httprouter.Router, secret string) {
	httpMux.Handler("GET", "/_matrix/media/unstable/io.t2bot/debug/pprof/", pprofServe(pprof.Index, secret))
	httpMux.Handler("GET", "/_matrix/media/unstable/io.t2bot/debug/pprof/allocs", pprofServe(pprof.Index, secret))
	httpMux.Handler("GET", "/_matrix/media/unstable/io.t2bot/debug/pprof/block", pprofServe(pprof.Index, secret))
	httpMux.Handler("GET", "/_matrix/media/unstable/io.t2bot/debug/pprof/cmdline", pprofServe(pprof.Index, secret))
	httpMux.Handler("GET", "/_matrix/media/unstable/io.t2bot/debug/pprof/goroutine", pprofServe(pprof.Index, secret))
	httpMux.Handler("GET", "/_matrix/media/unstable/io.t2bot/debug/pprof/heap", pprofServe(pprof.Index, secret))
	httpMux.Handler("GET", "/_matrix/media/unstable/io.t2bot/debug/pprof/mutex", pprofServe(pprof.Index, secret))
	httpMux.Handler("GET", "/_matrix/media/unstable/io.t2bot/debug/pprof/profile", pprofServe(pprof.Profile, secret))
	httpMux.Handler("GET", "/_matrix/media/unstable/io.t2bot/debug/pprof/threadcreate", pprofServe(pprof.Index, secret))
	httpMux.Handler("GET", "/_matrix/media/unstable/io.t2bot/debug/pprof/trace", pprofServe(pprof.Trace, secret))
}

type generatorFn = func(w http.ResponseWriter, r *http.Request)

type requestContainer struct {
	secret string
	fn     generatorFn
}

func pprofServe(fn generatorFn, secret string) http.Handler {
	return &requestContainer{
		secret: secret,
		fn:     fn,
	}
}

func (c *requestContainer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		auth = r.URL.Query().Get("access_token")
	}
	if auth != ("Bearer " + c.secret) {
		// Order is important: Set headers before sending responses
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusUnauthorized)

		encoder := json.NewEncoder(w)
		_ = encoder.Encode(&map[string]bool{"success": false})
		return
	}

	// otherwise authed fine
	r.URL.Path = r.URL.Path[len("/_matrix/media/unstable/io.t2bot"):]
	c.fn(w, r)
}
