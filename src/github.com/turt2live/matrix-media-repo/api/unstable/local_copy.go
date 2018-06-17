package unstable

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/api/r0"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/controllers/download_controller"
	"github.com/turt2live/matrix-media-repo/controllers/upload_controller"
)

func LocalCopy(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	params := mux.Vars(r)

	server := params["server"]
	mediaId := params["mediaId"]
	allowRemote := r.URL.Query().Get("allow_remote")

	downloadRemote := true
	if allowRemote != "" {
		parsedFlag, err := strconv.ParseBool(allowRemote)
		if err != nil {
			return api.InternalServerError("allow_remote flag does not appear to be a boolean")
		}
		downloadRemote = parsedFlag
	}

	log = log.WithFields(logrus.Fields{
		"mediaId":     mediaId,
		"server":      server,
		"allowRemote": downloadRemote,
	})

	// TODO: There's a lot of room for improvement here. Instead of re-uploading media, we should just update the DB.

	streamedMedia, err := download_controller.GetMedia(server, mediaId, downloadRemote, r.Context(), log)
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
	defer streamedMedia.Stream.Close()

	// Don't clone the media if it's already available on this domain
	if streamedMedia.Media.Origin == r.Host {
		return &r0.MediaUploadedResponse{ContentUri: streamedMedia.Media.MxcUri()}
	}

	newMedia, err := upload_controller.UploadMedia(streamedMedia.Stream, streamedMedia.Media.ContentType, streamedMedia.Media.UploadName, user.UserId, r.Host, r.Context(), log)
	if err != nil {
		if err == common.ErrMediaNotAllowed {
			return api.BadRequest("Media content type not allowed on this server")
		}

		log.Error("Unexpected error storing media: " + err.Error())
		return api.InternalServerError("Unexpected Error")
	}

	return &r0.MediaUploadedResponse{ContentUri: newMedia.MxcUri()}
}
