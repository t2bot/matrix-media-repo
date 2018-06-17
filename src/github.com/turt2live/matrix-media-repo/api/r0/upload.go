package r0

import (
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/controllers/upload_controller"
)

type MediaUploadedResponse struct {
	ContentUri   string  `json:"content_uri"`
	ContentToken *string `json:"content_token,omitempty"`
}

func UploadMedia(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		filename = "upload.bin"
	}

	isPublicStr := r.URL.Query().Get("public")
	isPublic := true
	if isPublicStr != "" {
		parsedFlag, err := strconv.ParseBool(isPublicStr)
		if err != nil {
			return api.InternalServerError("public flag does not appear to be a boolean")
		}

		isPublic = parsedFlag
	}

	log = log.WithFields(logrus.Fields{
		"filename": filename,
		"isPublic": isPublic,
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

	media, err := upload_controller.UploadMedia(r.Body, contentType, filename, user.UserId, r.Host, isPublic, r.Context(), log)
	if err != nil {
		io.Copy(ioutil.Discard, r.Body) // Ditch the entire request
		defer r.Body.Close()

		if err == common.ErrMediaNotAllowed {
			return api.BadRequest("Media content type not allowed on this server")
		}

		log.Error("Unexpected error storing media: " + err.Error())
		return api.InternalServerError("Unexpected Error")
	}

	return &MediaUploadedResponse{ContentUri: media.MxcUri(), ContentToken: media.ContentToken}
}
