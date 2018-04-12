package r0

import (
	"context"
	"database/sql"
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
	NumQuarantined int `json:"num_quarantined"`
}

func QuarantineRoomMedia(r *http.Request, log *logrus.Entry, user userInfo) interface{} {
	canQuarantine, allowOtherHosts, isLocalAdmin := getQuarantineRequestInfo(r, log, user)
	if !canQuarantine {
		return client.AuthFailed()
	}

	params := mux.Vars(r)

	roomId := params["roomId"]

	log = log.WithFields(logrus.Fields{
		"roomId":     roomId,
		"localAdmin": isLocalAdmin,
	})

	allMedia, err := matrix.ListMedia(r.Context(), r.Host, user.accessToken, roomId)
	if err != nil {
		log.Error("Error while listing media in the room: " + err.Error())
		return client.InternalServerError("error retrieving media in room")
	}

	var mxcs []string
	mxcs = append(mxcs, allMedia.LocalMxcs...)
	mxcs = append(mxcs, allMedia.RemoteMxcs...)

	total := 0
	for _, mxc := range mxcs {
		server, mediaId, err := util.SplitMxc(mxc)
		if err != nil {
			log.Error("Error parsing MXC URI (" + mxc + "): " + err.Error())
			return client.InternalServerError("error parsing mxc uri")
		}

		if !allowOtherHosts && r.Host != server {
			log.Warn("Skipping media " + mxc + " because it is on a different host")
			continue
		}

		resp, ok := doQuarantine(r.Context(), log, server, mediaId, allowOtherHosts)
		if !ok {
			return resp
		}

		total += resp.(*MediaQuarantinedResponse).NumQuarantined
	}

	return &MediaQuarantinedResponse{NumQuarantined: total}
}

func QuarantineMedia(r *http.Request, log *logrus.Entry, user userInfo) interface{} {
	canQuarantine, allowOtherHosts, isLocalAdmin := getQuarantineRequestInfo(r, log, user)
	if !canQuarantine {
		return client.AuthFailed()
	}

	params := mux.Vars(r)

	server := params["server"]
	mediaId := params["mediaId"]

	log = log.WithFields(logrus.Fields{
		"server":     server,
		"mediaId":    mediaId,
		"localAdmin": isLocalAdmin,
	})

	if !allowOtherHosts && r.Host != server {
		return client.BadRequest("unable to quarantine media on other homeservers")
	}

	resp, _ := doQuarantine(r.Context(), log, server, mediaId, allowOtherHosts)
	return resp
}

func doQuarantine(ctx context.Context, log *logrus.Entry, server string, mediaId string, allowOtherHosts bool) (interface{}, bool) {
	// We don't bother clearing the cache because it's still probably useful there
	mediaSvc := media_service.New(ctx, log)
	media, err := mediaSvc.GetMediaDirect(server, mediaId)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Warn("Media not found, could not quarantine: " + server + "/" + mediaId)
			return &MediaQuarantinedResponse{0}, true
		}

		log.Error("Error fetching media: " + err.Error())
		return client.InternalServerError("error quarantining media"), false
	}

	num, err := mediaSvc.SetMediaQuarantined(media, true, allowOtherHosts)
	if err != nil {
		log.Error("Error quarantining media: " + err.Error())
		return client.InternalServerError("Error quarantining media"), false
	}

	return &MediaQuarantinedResponse{NumQuarantined: num}, true
}

func getQuarantineRequestInfo(r *http.Request, log *logrus.Entry, user userInfo) (bool, bool, bool) {
	isGlobalAdmin := util.IsGlobalAdmin(user.userId)
	canQuarantine := isGlobalAdmin
	allowOtherHosts := isGlobalAdmin
	isLocalAdmin := false
	var err error
	if !isGlobalAdmin {
		if config.Get().Quarantine.AllowLocalAdmins {
			isLocalAdmin, err = matrix.IsUserAdmin(r.Context(), r.Host, user.accessToken)
			if err != nil {
				log.Error("Error verifying local admin: " + err.Error())
				canQuarantine = false
				return canQuarantine, allowOtherHosts, isLocalAdmin
			}

			if !isLocalAdmin {
				log.Warn(user.userId + " tried to quarantine media on another server")
				canQuarantine = false
				return canQuarantine, allowOtherHosts, isLocalAdmin
			}

			// They have local admin status and we allow local admins to quarantine
			canQuarantine = true
		}
	}

	if !canQuarantine {
		log.Warn(user.userId + " tried to quarantine media")
	}

	return canQuarantine, allowOtherHosts, isLocalAdmin
}
