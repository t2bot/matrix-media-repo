package r0

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/media_cache"
	"github.com/turt2live/matrix-media-repo/services/media_service"
	"github.com/turt2live/matrix-media-repo/util/errs"
)

func LocalCopy(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	params := mux.Vars(r)

	server := params["server"]
	mediaId := params["mediaId"]

	log = log.WithFields(logrus.Fields{
		"mediaId": mediaId,
		"server":  server,
	})

	// TODO: There's a lot of room for improvement here. Instead of re-uploading media, we should just update the DB.

	mediaCache := media_cache.Create(r.Context(), log)
	svc := media_service.New(r.Context(), log)

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

	// Don't clone the media if it's already available on this domain
	if streamedMedia.Media.Origin == r.Host {
		return &MediaUploadedResponse{streamedMedia.Media.MxcUri()}
	}

	newMedia, err := svc.StoreMedia(streamedMedia.Stream, streamedMedia.Media.ContentType, streamedMedia.Media.UploadName, user.UserId, r.Host, "")
	if err != nil {
		if err == errs.ErrMediaNotAllowed {
			return api.BadRequest("Media content type not allowed on this server")
		}

		log.Error("Unexpected error storing media: " + err.Error())
		return api.InternalServerError("Unexpected Error")
	}

	return &MediaUploadedResponse{newMedia.MxcUri()}
}
