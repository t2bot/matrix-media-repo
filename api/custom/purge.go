package custom

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/api/_routers"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/maintenance_controller"
	"github.com/turt2live/matrix-media-repo/matrix"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type MediaPurgedResponse struct {
	NumRemoved int `json:"total_removed"`
}

func PurgeRemoteMedia(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	beforeTsStr := r.URL.Query().Get("before_ts")
	if beforeTsStr == "" {
		return _responses.BadRequest("Missing before_ts argument")
	}
	beforeTs, err := strconv.ParseInt(beforeTsStr, 10, 64)
	if err != nil {
		return _responses.BadRequest("Error parsing before_ts: " + err.Error())
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"beforeTs": beforeTs,
	})

	// We don't bother clearing the cache because it's still probably useful there
	removed, err := maintenance_controller.PurgeRemoteMediaBefore(beforeTs, rctx)
	if err != nil {
		rctx.Log.Error("Error purging remote media: " + err.Error())
		sentry.CaptureException(err)
		return _responses.InternalServerError("Error purging remote media")
	}

	return &_responses.DoNotCacheResponse{Payload: &MediaPurgedResponse{NumRemoved: removed}}
}

func PurgeIndividualRecord(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	isGlobalAdmin, isLocalAdmin := _apimeta.GetRequestUserAdminStatus(r, rctx, user)
	localServerName := r.Host

	server := _routers.GetParam("server", r)
	mediaId := _routers.GetParam("mediaId", r)

	if !_routers.ServerNameRegex.MatchString(server) {
		return _responses.BadRequest("invalid server ID")
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"server":  server,
		"mediaId": mediaId,
	})

	// If the user is NOT a global admin, ensure they are speaking to the right server
	if !isGlobalAdmin {
		if server != localServerName {
			return _responses.AuthFailed()
		}
		// If the user is NOT a local admin, ensure they uploaded the content in the first place
		if !isLocalAdmin {
			db := storage.GetDatabase().GetMediaStore(rctx)
			m, err := db.Get(server, mediaId)
			if err == sql.ErrNoRows {
				return _responses.NotFoundError()
			}
			if err != nil {
				rctx.Log.Error("Error checking ownership of media: " + err.Error())
				sentry.CaptureException(err)
				return _responses.InternalServerError("error checking media ownership")
			}
			if m.UserId != user.UserId {
				return _responses.AuthFailed()
			}
		}
	}

	err := maintenance_controller.PurgeMedia(server, mediaId, rctx)
	if err == sql.ErrNoRows || err == common.ErrMediaNotFound {
		return _responses.NotFoundError()
	}
	if err != nil {
		rctx.Log.Error("Error purging media: " + err.Error())
		sentry.CaptureException(err)
		return _responses.InternalServerError("error purging media")
	}

	return &_responses.DoNotCacheResponse{Payload: map[string]interface{}{"purged": true}}
}

func PurgeQuarantined(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	isGlobalAdmin, isLocalAdmin := _apimeta.GetRequestUserAdminStatus(r, rctx, user)
	localServerName := r.Host

	var affected []*types.Media
	var err error

	if isGlobalAdmin {
		affected, err = maintenance_controller.PurgeQuarantined(rctx)
	} else if isLocalAdmin {
		affected, err = maintenance_controller.PurgeQuarantinedFor(localServerName, rctx)
	} else {
		return _responses.AuthFailed()
	}

	if err != nil {
		rctx.Log.Error("Error purging media: " + err.Error())
		sentry.CaptureException(err)
		return _responses.InternalServerError("error purging media")
	}

	mxcs := make([]string, 0)
	for _, a := range affected {
		mxcs = append(mxcs, a.MxcUri())
	}

	return &_responses.DoNotCacheResponse{Payload: map[string]interface{}{"purged": true, "affected": mxcs}}
}

func PurgeOldMedia(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	var err error
	beforeTs := util.NowMillis()
	beforeTsStr := r.URL.Query().Get("before_ts")
	if beforeTsStr != "" {
		beforeTs, err = strconv.ParseInt(beforeTsStr, 10, 64)
		if err != nil {
			return _responses.BadRequest("Error parsing before_ts: " + err.Error())
		}
	}

	includeLocal := false
	includeLocalStr := r.URL.Query().Get("include_local")
	if includeLocalStr != "" {
		includeLocal, err = strconv.ParseBool(includeLocalStr)
		if err != nil {
			return _responses.BadRequest("Error parsing include_local: " + err.Error())
		}
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"before_ts":     beforeTs,
		"include_local": includeLocal,
	})

	affected, err := maintenance_controller.PurgeOldMedia(beforeTs, includeLocal, rctx)

	if err != nil {
		rctx.Log.Error("Error purging media: " + err.Error())
		sentry.CaptureException(err)
		return _responses.InternalServerError("error purging media")
	}

	mxcs := make([]string, 0)
	for _, a := range affected {
		mxcs = append(mxcs, a.MxcUri())
	}

	return &_responses.DoNotCacheResponse{Payload: map[string]interface{}{"purged": true, "affected": mxcs}}
}

func PurgeUserMedia(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	isGlobalAdmin, isLocalAdmin := _apimeta.GetRequestUserAdminStatus(r, rctx, user)
	if !isGlobalAdmin && !isLocalAdmin {
		return _responses.AuthFailed()
	}

	var err error
	beforeTs := util.NowMillis()
	beforeTsStr := r.URL.Query().Get("before_ts")
	if beforeTsStr != "" {
		beforeTs, err = strconv.ParseInt(beforeTsStr, 10, 64)
		if err != nil {
			return _responses.BadRequest("Error parsing before_ts: " + err.Error())
		}
	}

	userId := _routers.GetParam("userId", r)

	rctx = rctx.LogWithFields(logrus.Fields{
		"userId":   userId,
		"beforeTs": beforeTs,
	})

	_, userDomain, err := util.SplitUserId(userId)
	if err != nil {
		rctx.Log.Error("Error parsing user ID (" + userId + "): " + err.Error())
		sentry.CaptureException(err)
		return _responses.InternalServerError("error parsing user ID")
	}

	if !isGlobalAdmin && userDomain != r.Host {
		return _responses.AuthFailed()
	}

	affected, err := maintenance_controller.PurgeUserMedia(userId, beforeTs, rctx)

	if err != nil {
		rctx.Log.Error("Error purging media: " + err.Error())
		sentry.CaptureException(err)
		return _responses.InternalServerError("error purging media")
	}

	mxcs := make([]string, 0)
	for _, a := range affected {
		mxcs = append(mxcs, a.MxcUri())
	}

	return &_responses.DoNotCacheResponse{Payload: map[string]interface{}{"purged": true, "affected": mxcs}}
}

func PurgeRoomMedia(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	isGlobalAdmin, isLocalAdmin := _apimeta.GetRequestUserAdminStatus(r, rctx, user)
	if !isGlobalAdmin && !isLocalAdmin {
		return _responses.AuthFailed()
	}

	var err error
	beforeTs := util.NowMillis()
	beforeTsStr := r.URL.Query().Get("before_ts")
	if beforeTsStr != "" {
		beforeTs, err = strconv.ParseInt(beforeTsStr, 10, 64)
		if err != nil {
			return _responses.BadRequest("Error parsing before_ts: " + err.Error())
		}
	}

	roomId := _routers.GetParam("roomId", r)

	rctx = rctx.LogWithFields(logrus.Fields{
		"roomId":   roomId,
		"beforeTs": beforeTs,
	})

	allMedia, err := matrix.ListMedia(rctx, r.Host, user.AccessToken, roomId, r.RemoteAddr)
	if err != nil {
		rctx.Log.Error("Error while listing media in the room: " + err.Error())
		sentry.CaptureException(err)
		return _responses.InternalServerError("error retrieving media in room")
	}

	mxcs := make([]string, 0)
	if !isGlobalAdmin {
		for _, mxc := range allMedia.LocalMxcs {
			domain, _, err := util.SplitMxc(mxc)
			if err != nil {
				continue
			}
			if domain != r.Host {
				continue
			}
			mxcs = append(mxcs, mxc)
		}

		for _, mxc := range allMedia.RemoteMxcs {
			domain, _, err := util.SplitMxc(mxc)
			if err != nil {
				continue
			}
			if domain != r.Host {
				continue
			}
			mxcs = append(mxcs, mxc)
		}
	} else {
		for _, mxc := range allMedia.LocalMxcs {
			mxcs = append(mxcs, mxc)
		}
		for _, mxc := range allMedia.RemoteMxcs {
			mxcs = append(mxcs, mxc)
		}
	}

	affected, err := maintenance_controller.PurgeRoomMedia(mxcs, beforeTs, rctx)

	if err != nil {
		rctx.Log.Error("Error purging media: " + err.Error())
		sentry.CaptureException(err)
		return _responses.InternalServerError("error purging media")
	}

	mxcs = make([]string, 0)
	for _, a := range affected {
		mxcs = append(mxcs, a.MxcUri())
	}

	return &_responses.DoNotCacheResponse{Payload: map[string]interface{}{"purged": true, "affected": mxcs}}
}

func PurgeDomainMedia(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	isGlobalAdmin, isLocalAdmin := _apimeta.GetRequestUserAdminStatus(r, rctx, user)
	if !isGlobalAdmin && !isLocalAdmin {
		return _responses.AuthFailed()
	}

	var err error
	beforeTs := util.NowMillis()
	beforeTsStr := r.URL.Query().Get("before_ts")
	if beforeTsStr != "" {
		beforeTs, err = strconv.ParseInt(beforeTsStr, 10, 64)
		if err != nil {
			return _responses.BadRequest("Error parsing before_ts: " + err.Error())
		}
	}

	serverName := _routers.GetParam("serverName", r)

	if !_routers.ServerNameRegex.MatchString(serverName) {
		return _responses.BadRequest("invalid server name")
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"serverName": serverName,
		"beforeTs":   beforeTs,
	})

	if !isGlobalAdmin && serverName != r.Host {
		return _responses.AuthFailed()
	}

	affected, err := maintenance_controller.PurgeDomainMedia(serverName, beforeTs, rctx)

	if err != nil {
		rctx.Log.Error("Error purging media: " + err.Error())
		sentry.CaptureException(err)
		return _responses.InternalServerError("error purging media")
	}

	mxcs := make([]string, 0)
	for _, a := range affected {
		mxcs = append(mxcs, a.MxcUri())
	}

	return &_responses.DoNotCacheResponse{Payload: map[string]interface{}{"purged": true, "affected": mxcs}}
}
