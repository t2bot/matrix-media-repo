package r0

import (
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/client"
	"github.com/turt2live/matrix-media-repo/rcontext"
	"github.com/turt2live/matrix-media-repo/services"
	"github.com/turt2live/matrix-media-repo/util"
)

type MediaUploadedResponse struct {
	ContentUri string `json:"content_uri"`
}

func UploadMedia(w http.ResponseWriter, r *http.Request, i rcontext.RequestInfo) interface{} {
	accessToken := util.GetAccessTokenFromRequest(r)
	userId, err := util.GetUserIdFromToken(r.Context(), r.Host, accessToken, i.Config)
	if err != nil || userId == "" {
		return client.AuthFailed()
	}

	filename := r.URL.Query().Get("filename")
	if filename == "" {
		filename = "upload.bin"
	}

	i.Log = i.Log.WithFields(logrus.Fields{
		"filename": filename,
		"userId":   userId,
	})

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream" // binary
	}

	svc := services.CreateMediaService(i)

	if svc.IsTooLarge(r.ContentLength, r.Header.Get("Content-Length")) {
		return client.RequestTooLarge()
	}

	media, err := svc.UploadMedia(r.Body, contentType, filename, userId, r.Host)
	if err != nil {
		i.Log.Error("Unexpected error storing media: " + err.Error())
		return client.InternalServerError("Unexpected Error")
	}

	mxc := util.MediaToMxc(&media)
	return &MediaUploadedResponse{mxc}
}
