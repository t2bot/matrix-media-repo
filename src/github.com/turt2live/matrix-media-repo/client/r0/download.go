package r0

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/storage"
)

// Request:
//   Path params: {serverName}, {mediaId}
//   Optional path param: {filename}
//
// Response:
//   Headers: Content-Type, Content-Disposition
//   Body: <byte[]>

type DownloadMediaResponse struct {
	Server string
	MediaID string
	Filename string
	Host string
}

func DownloadMedia(w http.ResponseWriter, r *http.Request, db storage.Database, c config.MediaRepoConfig) interface{} {
	params := mux.Vars(r)

	server := params["server"]
	mediaId := params["mediaId"]
	filename := params["filename"]

	if filename == "" {
		filename = "testasdasdasd.jpg"
	}

	return &DownloadMediaResponse{server, mediaId, filename, r.Host}
}
