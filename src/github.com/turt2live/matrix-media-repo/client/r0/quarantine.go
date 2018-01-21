package r0

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/client"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/matrix"
	"github.com/turt2live/matrix-media-repo/services/media_service"
	"github.com/turt2live/matrix-media-repo/util"
)

type MediaQuarantinedResponse struct {
	IsQuarantined bool `json:"is_quarantined"`
}

func QuarantineMedia(w http.ResponseWriter, r *http.Request, log *logrus.Entry) interface{} {
	accessToken := util.GetAccessTokenFromRequest(r)
	userId, err := matrix.GetUserIdFromToken(r.Context(), r.Host, accessToken)
	if err != nil || userId == "" {
		if err != nil {
			log.Error("Error verifying token: " + err.Error())
		}
		return client.AuthFailed()
	}
	isGlobalAdmin := util.IsGlobalAdmin(userId)
	canQuarantine := isGlobalAdmin
	allowOtherHosts := isGlobalAdmin
	isLocalAdmin := false
	if !isGlobalAdmin {
		if config.Get().Quarantine.AllowLocalAdmins {
			isLocalAdmin, err = matrix.IsUserAdmin(r.Context(), r.Host, accessToken)
			if err != nil {
				log.Error("Error verifying local admin: " + err.Error())
				return client.AuthFailed()
			}

			if !isLocalAdmin {
				log.Warn(userId + " tried to quarantine media on another server")
				return client.AuthFailed()
			}

			// They have local admin status and we allow local admins to quarantine
			canQuarantine = true
		}
	}

	if !canQuarantine {
		log.Warn(userId + " tried to quarantine media")
		return client.AuthFailed()
	}

	params := mux.Vars(r)

	server := params["server"]
	mediaId := params["mediaId"]

	log = log.WithFields(logrus.Fields{
		"server":      server,
		"mediaId":     mediaId,
		"userId":      userId,
		"localAdmin":  isLocalAdmin,
		"globalAdmin": isGlobalAdmin,
	})

	if !allowOtherHosts && r.Host != server {
		return client.BadRequest("unable to quarantine media on other homeservers")
	}

	// We don't bother clearing the cache because it's still probably useful there
	mediaSvc := media_service.New(r.Context(), log)
	media, err := mediaSvc.GetMediaDirect(server, mediaId)
	if err != nil {
		log.Error("Error fetching media: " + err.Error())
		return client.BadRequest("media not found or other error encountered - see logs")
	}

	err = mediaSvc.SetMediaQuarantined(media, true, allowOtherHosts)
	if err != nil {
		log.Error("Error quarantining media: " + err.Error())
		return client.InternalServerError("Error quarantining media")
	}

	return &MediaQuarantinedResponse{
		IsQuarantined: true,
	}
}
