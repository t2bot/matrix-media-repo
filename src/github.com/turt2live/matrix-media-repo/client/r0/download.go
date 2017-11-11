package r0

import (
	"net/http"
	"github.com/gorilla/mux"
	"io"
)

// Request:
//   Path params: {serverName}, {mediaId}
//   Optional path param: {filename}
//
// Response:
//   Headers: Content-Type, Content-Disposition
//   Body: <byte[]>

func DownloadMedia(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	server := params["server"]
	mediaId := params["mediaId"]
	filename := params["filename"]

	if filename == "" {
		filename = "testasdasdasd.jpg"
	}

	io.WriteString(w, "Server = "+server+"; mediaId = "+mediaId+"; filename = "+filename+"; Host = "+r.Host)
}
