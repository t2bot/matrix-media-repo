package r0

import (
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/media_cache"
	"github.com/turt2live/matrix-media-repo/util"
)

type DownloadMediaResponse struct {
	ContentType string
	Filename    string
	SizeBytes   int64
	Data        io.ReadCloser
}

func DownloadMedia(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	hs := util.GetHomeserverConfig(r.Host)
	if hs.DownloadRequiresAuth && user.UserId == "" {
		log.Warn("Homeserver requires authenticated downloads - denying request")
		return api.AuthFailed()
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

	mediaCache := media_cache.Create(r.Context(), log)

	streamedMedia, err := mediaCache.GetMedia(server, mediaId)
	if err != nil {
		if err == common.ErrMediaNotFound {
			return api.NotFoundError()
		} else if err == common.ErrMediaTooLarge {
			return api.RequestTooLarge()
		} else if err == common.ErrMediaQuarantined {
			return api.NotFoundError() // We lie for security
		}
		log.Error("Unexpected error locating media: " + err.Error())
		return api.InternalServerError("Unexpected Error")
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
