package r0

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/turt2live/matrix-media-repo/client"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/media_handler"
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

	media, err := media_handler.FindMedia(r.Context(), server, mediaId, c, db)
	if err != nil {
		if err == media_handler.ErrMediaNotFound {
			return client.NotFoundError()
		} else if err == media_handler.ErrMediaTooLarge {
			return client.RequestTooLarge()
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
