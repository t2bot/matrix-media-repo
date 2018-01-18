package r0

import (
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/client"
	"github.com/turt2live/matrix-media-repo/services/media_service"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/errs"
)

type DownloadMediaResponse struct {
	ContentType string
	Filename    string
	SizeBytes   int64
	Data        io.ReadCloser
}

func DownloadMedia(w http.ResponseWriter, r *http.Request, log *logrus.Entry) interface{} {
	if !ValidateUserCanDownload(r, log) {
		return client.AuthFailed()
	}

	params := mux.Vars(r)

	server := params["server"]
	mediaId := params["mediaId"]
	filename := params["filename"]

	log = log.WithFields(logrus.Fields{
		"mediaId":  mediaId,
		"server":   server,
		"filename": filename,
	})

	svc := media_service.New(r.Context(), log)

	streamedMedia, err := svc.GetStreamedMedia(server, mediaId)
	if err != nil {
		if err == errs.ErrMediaNotFound {
			return client.NotFoundError()
		} else if err == errs.ErrMediaTooLarge {
			return client.RequestTooLarge()
		}
		log.Error("Unexpected error locating media: " + err.Error())
		return client.InternalServerError("Unexpected Error")
	}

	if filename == "" {
		filename = streamedMedia.Media.UploadName
	}

	return &DownloadMediaResponse{
		ContentType: streamedMedia.Media.ContentType,
		Filename:    filename,
		SizeBytes:   streamedMedia.Media.SizeBytes,
		Data:        streamedMedia.Stream,
	}
}

func ValidateUserCanDownload(r *http.Request, log *logrus.Entry) (bool) {
	hs := util.GetHomeserverConfig(r.Host)
	if !hs.DownloadRequiresAuth {
		return true // no auth required == can access
	}

	accessToken := util.GetAccessTokenFromRequest(r)
	userId, err := util.GetUserIdFromToken(r.Context(), r.Host, accessToken)
	if err != nil {
		log.Error("Error verifying token: " + err.Error())
	}
	return userId != "" && err != nil
}
