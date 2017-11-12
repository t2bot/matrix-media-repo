package r0

import (
	"database/sql"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/turt2live/matrix-media-repo/client"
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
	ContentType string
	Filename string
	SizeBytes int64
	Location string
}

func DownloadMedia(w http.ResponseWriter, r *http.Request, db storage.Database, c config.MediaRepoConfig) interface{} {
	params := mux.Vars(r)

	server := params["server"]
	mediaId := params["mediaId"]
	filename := params["filename"]

	media, err := db.GetMedia(r.Context(), server, mediaId)
	if err != nil {
		if err == sql.ErrNoRows {
			// TODO: Try remote fetch
			return client.NotFoundError()
		}
		return client.InternalServerError(err.Error())
	}

	if filename == "" {
		filename = media.UploadName
	}

	return &DownloadMediaResponse{
		ContentType: media.ContentType,
		Filename:    filename,
		SizeBytes:   media.SizeBytes,
		Location:    media.Location,
	}
}
