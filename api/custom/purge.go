package custom

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/controllers/maintenance_controller"
)

type MediaPurgedResponse struct {
	NumRemoved int `json:"total_removed"`
}

func PurgeRemoteMedia(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	beforeTsStr := r.URL.Query().Get("before_ts")
	if beforeTsStr == "" {
		return api.BadRequest("Missing before_ts argument")
	}
	beforeTs, err := strconv.ParseInt(beforeTsStr, 10, 64)
	if err != nil {
		return api.BadRequest("Error parsing before_ts: " + err.Error())
	}

	log = log.WithFields(logrus.Fields{
		"beforeTs": beforeTs,
	})

	// We don't bother clearing the cache because it's still probably useful there
	removed, err := maintenance_controller.PurgeRemoteMediaBefore(beforeTs, r.Context(), log)
	if err != nil {
		log.Error("Error purging remote media: " + err.Error())
		return api.InternalServerError("Error purging remote media")
	}

	return &api.DoNotCacheResponse{Payload: &MediaPurgedResponse{NumRemoved: removed}}
}

func PurgeIndividualRecord(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	// TODO: Allow non-repo-admins to delete things

	params := mux.Vars(r)

	server := params["server"]
	mediaId := params["mediaId"]

	log = log.WithFields(logrus.Fields{
		"server":  server,
		"mediaId": mediaId,
	})

	err := maintenance_controller.PurgeMedia(server, mediaId, r.Context(), log)
	if err != nil {
		log.Error("Error purging media: " + err.Error())
		return api.InternalServerError("error purging media")
	}

	return &api.DoNotCacheResponse{Payload: map[string]interface{}{"purged": true}}
}

func PurgeQurantined(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	// TODO: Allow non-repo-admins to delete things

	affected, err := maintenance_controller.PurgeQuarantined(r.Context(), log)
	if err != nil {
		log.Error("Error purging media: " + err.Error())
		return api.InternalServerError("error purging media")
	}

	mxcs := make([]string, 0)
	for _, a := range affected {
		mxcs = append(mxcs, a.MxcUri())
	}

	return &api.DoNotCacheResponse{Payload: map[string]interface{}{"purged": true, "affected": mxcs}}
}
