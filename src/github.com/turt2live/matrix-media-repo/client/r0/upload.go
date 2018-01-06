package r0

import (
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/client"
	"github.com/turt2live/matrix-media-repo/rcontext"
	"github.com/turt2live/matrix-media-repo/services/handlers"
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

	var reader io.Reader
	reader = r.Body
	if i.Config.Uploads.MaxSizeBytes > 0 {
		reader = io.LimitReader(r.Body, i.Config.Uploads.MaxSizeBytes)
	}

	request := &handlers.MediaUploadRequest{
		UploadedBy:      userId,
		ContentType:     contentType,
		DesiredFilename: filename,
		Host:            r.Host,
		Contents:        reader,
	}

	mxc, err := request.StoreAndGetMxcUri(i)
	if err != nil {
		i.Log.Error("Unexpected error storing media: " + err.Error())
		return client.InternalServerError("Unexpected Error")
	}

	return &MediaUploadedResponse{mxc}
}
