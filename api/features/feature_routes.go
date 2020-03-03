package features

import (
	"net/http"
)

const MSC2448UploadRoute = "/_matrix/media/unstable/xyz.amorgan/upload"
const MSC2448GetHashRoute = "/_matrix/media/unstable/xyz.amorgan/download/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[^/]+}/blurhash"
const MSC2448AltRenderRoute = "/_matrix/media/unstable/io.t2bot.msc2448/blurhash/{blurhash:[^/]+}"
const IPFSDownloadRoute = "/_matrix/media/unstable/io.t2bot.ipfs/download/{server:[a-zA-Z0-9.:\\-_]+}/{ipfsContentId:[^/]+}"
const IPFSLiveDownloadRouteR0 = "/_matrix/media/r0/download/{server:[a-zA-Z0-9.:\\-_]+}/ipfs:{ipfsContentId:[^/]+}"
const IPFSLiveDownloadRouteV1 = "/_matrix/media/v1/download/{server:[a-zA-Z0-9.:\\-_]+}/ipfs:{ipfsContentId:[^/]+}"
const IPFSLiveDownloadRouteUnstable = "/_matrix/media/v1/download/{server:[a-zA-Z0-9.:\\-_]+}/ipfs:{ipfsContentId:[^/]+}"

func IsRoute(r *http.Request, route string) bool {
	uri := r.URL.Path
	return uri == route
}
