package r0

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/client"
	"github.com/turt2live/matrix-media-repo/matrix"
	"github.com/turt2live/matrix-media-repo/media_cache"
	"github.com/turt2live/matrix-media-repo/services/media_service"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/errs"
)

func LocalCopy(w http.ResponseWriter, r *http.Request, log *logrus.Entry) interface{} {
	accessToken := util.GetAccessTokenFromRequest(r)
	appserviceUserId := util.GetAppserviceUserIdFromRequest(r)
	userId, err := matrix.GetUserIdFromToken(r.Context(), r.Host, accessToken, appserviceUserId)
	if err != nil || userId == "" {
		if err != nil {
			log.Error("Error verifying token: " + err.Error())
		}
		return client.AuthFailed()
	}

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
			return client.NotFoundError()
		} else if err == errs.ErrMediaTooLarge {
			return client.RequestTooLarge()
		} else if err == errs.ErrMediaQuarantined {
			return client.NotFoundError() // We lie for security
		}
		log.Error("Unexpected error locating media: " + err.Error())
		return client.InternalServerError("Unexpected Error")
	}
	defer streamedMedia.Stream.Close()

	// Don't clone the media if it's already available on this domain
	if streamedMedia.Media.Origin == r.Host {
		return &MediaUploadedResponse{streamedMedia.Media.MxcUri()}
	}

	newMedia, err := svc.StoreMedia(streamedMedia.Stream, streamedMedia.Media.ContentType, streamedMedia.Media.UploadName, userId, r.Host, "")
	if err != nil {
		if err == errs.ErrMediaNotAllowed {
			return client.BadRequest("Media content type not allowed on this server")
		}

		log.Error("Unexpected error storing media: " + err.Error())
		return client.InternalServerError("Unexpected Error")
	}

	return &MediaUploadedResponse{newMedia.MxcUri()}
}
