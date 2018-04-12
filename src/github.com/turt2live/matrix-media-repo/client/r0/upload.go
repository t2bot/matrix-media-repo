package r0

import (
	"io"
	"io/ioutil"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/client"
	"github.com/turt2live/matrix-media-repo/services/media_service"
	"github.com/turt2live/matrix-media-repo/util/errs"
)

type MediaUploadedResponse struct {
	ContentUri string `json:"content_uri"`
}

func UploadMedia(r *http.Request, log *logrus.Entry, user userInfo) interface{} {
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		filename = "upload.bin"
	}

	log = log.WithFields(logrus.Fields{
		"filename": filename,
	})

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream" // binary
	}

	svc := media_service.New(r.Context(), log)

	if svc.IsTooLarge(r.ContentLength, r.Header.Get("Content-Length")) {
		io.Copy(ioutil.Discard, r.Body) // Ditch the entire request
		defer r.Body.Close()
		return client.RequestTooLarge()
	}

	media, err := svc.UploadMedia(r.Body, contentType, filename, user.userId, r.Host)
	if err != nil {
		io.Copy(ioutil.Discard, r.Body) // Ditch the entire request
		defer r.Body.Close()

		if err == errs.ErrMediaNotAllowed {
			return client.BadRequest("Media content type not allowed on this server")
		}

		log.Error("Unexpected error storing media: " + err.Error())
		return client.InternalServerError("Unexpected Error")
	}

	return &MediaUploadedResponse{media.MxcUri()}
}
