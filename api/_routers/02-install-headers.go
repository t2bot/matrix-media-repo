package _routers

import (
	"net/http"
)

type InstallHeadersRouter struct {
	next http.Handler
}

func NewInstallHeadersRouter(next http.Handler) *InstallHeadersRouter {
	return &InstallHeadersRouter{next: next}
}

func (i *InstallHeadersRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	headers := w.Header()
	headers.Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
	headers.Set("Access-Control-Allow-Origin", "*")
	headers.Set("Content-Security-Policy", "sandbox; default-src 'none'; script-src 'none'; plugin-types application/pdf; style-src 'unsafe-inline'; media-src 'self'; object-src 'self';")
	headers.Set("Cross-Origin-Resource-Policy", "cross-origin")
	headers.Set("X-Content-Security-Policy", "sandbox;")
	headers.Set("X-Robots-Tag", "noindex, nofollow, noarchive, noimageindex")
	headers.Set("Server", "matrix-media-repo")

	if i.next != nil {
		i.next.ServeHTTP(w, r)
	}
}
