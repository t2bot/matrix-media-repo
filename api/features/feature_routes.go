package features

import (
	"net/http"
)

const MSC2448UploadRoute = "/_matrix/media/unstable/xyz.amorgan/upload"

func IsRoute(r *http.Request, route string) bool {
	uri := r.URL.Path
	return uri == route
}
