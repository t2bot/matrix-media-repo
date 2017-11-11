package r0

import "net/http"

// Request:
//   Path params: {serverName}, {mediaId}
//   QS: ?width=&height=&method=
//       "method" can be crop or scale
//
// Response:
//   Headers: Content-Type
//   Body: <byte[]>

func ThumbnailMedia(w http.ResponseWriter, r *http.Request) {

}
