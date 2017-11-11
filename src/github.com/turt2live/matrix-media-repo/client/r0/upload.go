package r0

import "net/http"

// Request:
//   QS: ?filename=
//   Headers: Content-Type
//   Body: <byte[]>
//
// Response:
//   Body: {"content_uri":"mxc://domain.com/media_id"}

func UploadMedia(w http.ResponseWriter, r *http.Request) {

}
