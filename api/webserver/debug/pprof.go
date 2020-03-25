package debug

import (
	"encoding/json"
	"net/http"
	"net/http/pprof"
)

func BindPprofEndpoints(httpMux *http.ServeMux, secret string) {
	httpMux.HandleFunc("/_matrix/media/unstable/io.t2bot/debug/pprof/", pprofServe(pprof.Index, secret))
	httpMux.HandleFunc("/_matrix/media/unstable/io.t2bot/debug/pprof/allocs", pprofServe(pprof.Index, secret))
	httpMux.HandleFunc("/_matrix/media/unstable/io.t2bot/debug/pprof/block", pprofServe(pprof.Index, secret))
	httpMux.HandleFunc("/_matrix/media/unstable/io.t2bot/debug/pprof/cmdline", pprofServe(pprof.Index, secret))
	httpMux.HandleFunc("/_matrix/media/unstable/io.t2bot/debug/pprof/goroutine", pprofServe(pprof.Index, secret))
	httpMux.HandleFunc("/_matrix/media/unstable/io.t2bot/debug/pprof/heap", pprofServe(pprof.Index, secret))
	httpMux.HandleFunc("/_matrix/media/unstable/io.t2bot/debug/pprof/mutex", pprofServe(pprof.Index, secret))
	httpMux.HandleFunc("/_matrix/media/unstable/io.t2bot/debug/pprof/profile", pprofServe(pprof.Index, secret))
	httpMux.HandleFunc("/_matrix/media/unstable/io.t2bot/debug/pprof/threadcreate", pprofServe(pprof.Index, secret))
	httpMux.HandleFunc("/_matrix/media/unstable/io.t2bot/debug/pprof/trace", pprofServe(pprof.Index, secret))
}

func pprofServe(fn func(http.ResponseWriter, *http.Request), secret string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != ("Bearer " + secret) {
			// Order is important: Set headers before sending responses
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusUnauthorized)

			encoder := json.NewEncoder(w)
			encoder.Encode(&map[string]bool{"success": false})
			return
		}

		// otherwise authed fine
		r.URL.Path = r.URL.Path[len("/_matrix/media/unstable/io.t2bot"):]
		fn(w, r)
	}
}
