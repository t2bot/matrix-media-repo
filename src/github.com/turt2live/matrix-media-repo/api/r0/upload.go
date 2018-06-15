package r0

import (
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/old_middle_layer/services/media_service"
)

type MediaUploadedResponse struct {
	ContentUri   string `json:"content_uri"`
	ContentToken string `json:"content_token,omitempty"`
}

func UploadMedia(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		filename = "upload.bin"
	}

	isPublic := r.URL.Query().Get("public")
	visibility := "public"
	if isPublic != "" {
		parsedFlag, err := strconv.ParseBool(isPublic)
		if err != nil {
			return api.InternalServerError("public flag does not appear to be a boolean")
		}

		if parsedFlag {
			visibility = "public"
		} else {
			visibility = "private"
		}
	}

	log = log.WithFields(logrus.Fields{
		"filename":   filename,
		"visibility": visibility,
	})

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream" // binary
	}

	svc := media_service.New(r.Context(), log)

	if svc.IsTooLarge(r.ContentLength, r.Header.Get("Content-Length")) {
		io.Copy(ioutil.Discard, r.Body) // Ditch the entire request
		defer r.Body.Close()
		return api.RequestTooLarge()
	}

	media, unhashedContentToken, err := svc.UploadMedia(r.Body, contentType, filename, visibility, user.UserId, r.Host)
	if err != nil {
		io.Copy(ioutil.Discard, r.Body) // Ditch the entire request
		defer r.Body.Close()

		if err == common.ErrMediaNotAllowed {
			return api.BadRequest("Media content type not allowed on this server")
		}

		log.Error("Unexpected error storing media: " + err.Error())
		return api.InternalServerError("Unexpected Error")
	}

	return &MediaUploadedResponse{media.MxcUri(), unhashedContentToken}
}
