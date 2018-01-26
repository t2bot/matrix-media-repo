package r0

import (
	"net/http"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/client"
	"github.com/turt2live/matrix-media-repo/matrix"
	"github.com/turt2live/matrix-media-repo/services/media_service"
	"github.com/turt2live/matrix-media-repo/util"
)

type MediaPurgedResponse struct {
	NumRemoved int `json:"total_removed"`
}

func PurgeRemoteMedia(w http.ResponseWriter, r *http.Request, log *logrus.Entry) interface{} {
	accessToken := util.GetAccessTokenFromRequest(r)
	appserviceUserId := util.GetAppserviceUserIdFromRequest(r)
	userId, err := matrix.GetUserIdFromToken(r.Context(), r.Host, accessToken, appserviceUserId)
	if err != nil || userId == "" {
		if err != nil {
			log.Error("Error verifying token: " + err.Error())
		}
		return client.AuthFailed()
	}
	isAdmin := util.IsGlobalAdmin(userId)
	if !isAdmin {
		log.Warn("User " + userId + " is not a repository administrator")
		return client.AuthFailed()
	}

	beforeTsStr := r.URL.Query().Get("before_ts")
	if beforeTsStr == "" {
		return client.BadRequest("Missing before_ts argument")
	}
	beforeTs, err := strconv.ParseInt(beforeTsStr, 10, 64)
	if err != nil {
		return client.BadRequest("Error parsing before_ts: " + err.Error())
	}

	log = log.WithFields(logrus.Fields{
		"beforeTs": beforeTs,
		"userId":   userId,
	})

	// We don't bother clearing the cache because it's still probably useful there
	mediaSvc := media_service.New(r.Context(), log)
	removed, err := mediaSvc.PurgeRemoteMediaBefore(beforeTs)
	if err != nil {
		log.Error("Error purging remote media: " + err.Error())
		return client.InternalServerError("Error purging remote media")
	}

	return &MediaPurgedResponse{
		NumRemoved: removed,
	}
}
