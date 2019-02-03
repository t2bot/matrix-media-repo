package r0

import (
	"io"
	"io/ioutil"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/controllers/upload_controller"
)

type MediaUploadedResponse struct {
	ContentUri string `json:"content_uri"`
}

func UploadMedia(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	filename := r.URL.Query().Get("filename")

	log = log.WithFields(logrus.Fields{
		"filename": filename,
	})

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream" // binary
	}

	if upload_controller.IsRequestTooLarge(r.ContentLength, r.Header.Get("Content-Length")) {
		io.Copy(ioutil.Discard, r.Body) // Ditch the entire request
		defer r.Body.Close()
		return api.RequestTooLarge()
	}

	if upload_controller.IsRequestTooSmall(r.ContentLength, r.Header.Get("Content-Length")) {
		io.Copy(ioutil.Discard, r.Body) // Ditch the entire request
		defer r.Body.Close()
		return api.RequestTooSmall()
	}

	media, err := upload_controller.UploadMedia(r.Body, contentType, filename, user.UserId, r.Host, r.Context(), log)
	if err != nil {
		io.Copy(ioutil.Discard, r.Body) // Ditch the entire request
		defer r.Body.Close()

		if err == common.ErrMediaNotAllowed {
			return api.BadRequest("Media content type not allowed on this server")
		}

		log.Error("Unexpected error storing media: " + err.Error())
		return api.InternalServerError("Unexpected Error")
	}

	return &MediaUploadedResponse{media.MxcUri()}
}
