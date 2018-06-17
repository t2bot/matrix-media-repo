package r0

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/controllers/download_controller"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type DownloadMediaResponse struct {
	ContentType string
	Filename    string
	SizeBytes   int64
	Data        io.ReadCloser
}

func DownloadMedia(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	params := mux.Vars(r)

	server := params["server"]
	mediaId := params["mediaId"]
	filename := params["filename"]
	allowRemote := r.URL.Query().Get("allow_remote")

	encryptedToken := util.GetMediaBearerTokenFromRequest(r)
	appserviceUserId := util.GetAppserviceUserIdFromRequest(r)
	bearerToken := &types.BearerToken{EncryptedToken: encryptedToken, AppserviceUserId: appserviceUserId}

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
		"filename":    filename,
		"allowRemote": downloadRemote,
	})

	streamedMedia, err := download_controller.GetMedia(server, mediaId, downloadRemote, bearerToken, r.Context(), log)
	if err != nil {
		if err == common.ErrMediaNotFound {
			return api.NotFoundError()
		} else if err == common.ErrMediaTooLarge {
			return api.RequestTooLarge()
		} else if err == common.ErrMediaQuarantined {
			return api.NotFoundError() // We lie for security
		} else if err == common.ErrFailedAuthCheck {
			return api.AuthFailed()
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
