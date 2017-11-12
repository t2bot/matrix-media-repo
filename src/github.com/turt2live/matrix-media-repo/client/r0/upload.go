package r0

import (
	"net/http"

	"github.com/turt2live/matrix-media-repo/client"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/media_handler"
	"github.com/turt2live/matrix-media-repo/storage"
)

// Request:
//   QS: ?filename=
//   Headers: Content-Type
//   Body: <byte[]>
//
// Response:
//   Body: {"content_uri":"mxc://domain.com/media_id"}

type MediaUploadedResponse struct {
	ContentUri string `json:"content_uri"`
}

func UploadMedia(w http.ResponseWriter, r *http.Request, db storage.Database, c config.MediaRepoConfig) interface{} {
	// TODO: Validate access_token

	filename := r.URL.Query().Get("filename")
	if filename == "" {
		filename = "upload.bin"
	}

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream" // binary
	}

	r.Body = http.MaxBytesReader(w, r.Body, c.Uploads.MaxSizeBytes)

	request := &media_handler.MediaUploadRequest{
		UploadedBy: "",
		ContentType: contentType,
		DesiredFilename:filename,
		Host:r.Host,
		Contents: r.Body,
	}

	mxc, err := request.StoreMedia(r.Context(), c, db)
	if err != nil {
		return client.InternalServerError(err.Error())
	}

	return &MediaUploadedResponse{mxc}
}