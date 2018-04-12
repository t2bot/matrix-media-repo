package r0

import (
	"net/http"

	"github.com/disintegration/imaging"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/media_cache"
	"github.com/turt2live/matrix-media-repo/util/errs"
)

type MediaInfoResponse struct {
	ContentUri  string `json:"content_uri"`
	ContentType string `json:"content_type"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	Size        int64  `json:"size"`
}

func MediaInfo(r *http.Request, log *logrus.Entry, user userInfo) interface{} {
	params := mux.Vars(r)

	server := params["server"]
	mediaId := params["mediaId"]

	log = log.WithFields(logrus.Fields{
		"mediaId": mediaId,
		"server":  server,
	})

	mediaCache := media_cache.Create(r.Context(), log)

	streamedMedia, err := mediaCache.GetMedia(server, mediaId)
	if err != nil {
		if err == errs.ErrMediaNotFound {
			return api.NotFoundError()
		} else if err == errs.ErrMediaTooLarge {
			return api.RequestTooLarge()
		} else if err == errs.ErrMediaQuarantined {
			return api.NotFoundError() // We lie for security
		}
		log.Error("Unexpected error locating media: " + err.Error())
		return api.InternalServerError("Unexpected Error")
	}
	defer streamedMedia.Stream.Close()

	response := &MediaInfoResponse{
		ContentUri:  streamedMedia.Media.MxcUri(),
		ContentType: streamedMedia.Media.ContentType,
		Size:        streamedMedia.Media.SizeBytes,
	}

	img, err := imaging.Decode(streamedMedia.Stream)
	if err == nil {
		response.Width = img.Bounds().Max.X
		response.Height = img.Bounds().Max.Y
	}

	return response
}
