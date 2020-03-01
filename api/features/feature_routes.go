package features

import (
	"net/http"
)

const MSC2448UploadRoute = "/_matrix/media/unstable/xyz.amorgan/upload"
const MSC2448AltRenderRoute = "/_matrix/media/unstable/io.t2bot.msc2448/blurhash/{blurhash:[^/]+}"

func IsRoute(r *http.Request, route string) bool {
	uri := r.URL.Path
	return uri == route
}
