package features

import (
	"net/http"
)

func IsRoute(r *http.Request, route string) bool {
	uri := r.URL.Path
	return uri == route
}
