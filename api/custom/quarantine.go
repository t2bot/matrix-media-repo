package custom

import (
	"errors"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/api/_routers"
	"github.com/t2bot/matrix-media-repo/api/apimeta"
	"github.com/t2bot/matrix-media-repo/api/responses"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/tasks/task_runner"

	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/matrix"
	"github.com/t2bot/matrix-media-repo/util"
)

type MediaQuarantinedResponse struct {
	NumQuarantined int64 `json:"num_quarantined"`
}

func QuarantineRoomMedia(r *http.Request, rctx rcontext.RequestContext, user apimeta.UserInfo) interface{} {
	canQuarantine, allowOtherHosts, isLocalAdmin := getQuarantineRequestInfo(r, rctx, user)
	if !canQuarantine {
		return responses.AuthFailed()
	}

	roomId := _routers.GetParam("roomId", r)

	rctx = rctx.LogWithFields(logrus.Fields{
		"roomId":     roomId,
		"localAdmin": isLocalAdmin,
	})

	allMedia, err := matrix.ListMedia(rctx, r.Host, user.AccessToken, roomId, r.RemoteAddr)
	if err != nil {
		rctx.Log.Error("Error while listing media in the room: ", err)
		sentry.CaptureException(err)
		return responses.InternalServerError(errors.New("error retrieving media in room"))
	}

	var mxcs []string
	mxcs = append(mxcs, allMedia.LocalMxcs...)
	if allowOtherHosts {
		mxcs = append(mxcs, allMedia.RemoteMxcs...)
	}

	return performQuarantineRequest(rctx, r.Host, allowOtherHosts, &task_runner.QuarantineThis{
		MxcUris: mxcs,
	})
}

func QuarantineUserMedia(r *http.Request, rctx rcontext.RequestContext, user apimeta.UserInfo) interface{} {
	canQuarantine, allowOtherHosts, isLocalAdmin := getQuarantineRequestInfo(r, rctx, user)
	if !canQuarantine {
		return responses.AuthFailed()
	}

	userId := _routers.GetParam("userId", r)

	rctx = rctx.LogWithFields(logrus.Fields{
		"userId":     userId,
		"localAdmin": isLocalAdmin,
	})

	_, userDomain, err := util.SplitUserId(userId)
	if err != nil {
		rctx.Log.Error("Error parsing user ID ("+userId+"): ", err)
		sentry.CaptureException(err)
		return responses.InternalServerError(errors.New("error parsing user ID"))
	}

	if !allowOtherHosts && userDomain != r.Host {
		return responses.AuthFailed()
	}

	db := database.GetInstance().Media.Prepare(rctx)
	userMedia, err := db.GetByUserId(userId)
	if err != nil {
		rctx.Log.Error("Error while listing media for the user: ", err)
		sentry.CaptureException(err)
		return responses.InternalServerError(errors.New("error retrieving media for user"))
	}

	return performQuarantineRequest(rctx, r.Host, allowOtherHosts, &task_runner.QuarantineThis{
		DbMedia: userMedia,
	})
}

func QuarantineDomainMedia(r *http.Request, rctx rcontext.RequestContext, user apimeta.UserInfo) interface{} {
	canQuarantine, allowOtherHosts, isLocalAdmin := getQuarantineRequestInfo(r, rctx, user)
	if !canQuarantine {
		return responses.AuthFailed()
	}

	serverName := _routers.GetParam("serverName", r)

	if !_routers.ServerNameRegex.MatchString(serverName) {
		return responses.BadRequest(errors.New("invalid server name"))
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"serverName": serverName,
		"localAdmin": isLocalAdmin,
	})

	if !allowOtherHosts && serverName != r.Host {
		return responses.AuthFailed()
	}

	db := database.GetInstance().Media.Prepare(rctx)
	domainMedia, err := db.GetByOrigin(serverName)
	if err != nil {
		rctx.Log.Error("Error while listing media for the server: ", err)
		sentry.CaptureException(err)
		return responses.InternalServerError(errors.New("error retrieving media for server"))
	}

	return performQuarantineRequest(rctx, r.Host, allowOtherHosts, &task_runner.QuarantineThis{
		DbMedia: domainMedia,
	})
}

func QuarantineMedia(r *http.Request, rctx rcontext.RequestContext, user apimeta.UserInfo) interface{} {
	canQuarantine, allowOtherHosts, isLocalAdmin := getQuarantineRequestInfo(r, rctx, user)
	if !canQuarantine {
		return responses.AuthFailed()
	}

	server := _routers.GetParam("server", r)
	mediaId := _routers.GetParam("mediaId", r)

	if !_routers.ServerNameRegex.MatchString(server) {
		return responses.BadRequest(errors.New("invalid server ID"))
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"server":     server,
		"mediaId":    mediaId,
		"localAdmin": isLocalAdmin,
	})

	if !allowOtherHosts && r.Host != server {
		return responses.BadRequest(errors.New("unable to quarantine media on other homeservers"))
	}

	return performQuarantineRequest(rctx, r.Host, allowOtherHosts, &task_runner.QuarantineThis{
		Single: &task_runner.QuarantineRecord{
			Origin:  server,
			MediaId: mediaId,
		},
	})
}

func performQuarantineRequest(ctx rcontext.RequestContext, host string, allowOtherHosts bool, toQuarantine *task_runner.QuarantineThis) interface{} {
	lockedHost := host
	if allowOtherHosts {
		lockedHost = ""
	}

	total, err := task_runner.QuarantineMedia(ctx, lockedHost, toQuarantine)
	if err != nil {
		ctx.Log.Error(err)
		sentry.CaptureException(err)
		return responses.InternalServerError(errors.New("error quarantining media"))
	}

	return &responses.DoNotCacheResponse{Payload: &MediaQuarantinedResponse{NumQuarantined: total}}
}

func getQuarantineRequestInfo(r *http.Request, rctx rcontext.RequestContext, user apimeta.UserInfo) (bool, bool, bool) {
	isGlobalAdmin := util.IsGlobalAdmin(user.UserId) || user.IsShared
	canQuarantine := isGlobalAdmin
	allowOtherHosts := isGlobalAdmin
	isLocalAdmin := false
	var err error
	if !isGlobalAdmin {
		if rctx.Config.Quarantine.AllowLocalAdmins {
			isLocalAdmin, err = matrix.IsUserAdmin(rctx, r.Host, user.AccessToken, r.RemoteAddr)
			if err != nil {
				sentry.CaptureException(err)
				rctx.Log.Debug("Error verifying local admin: ", err)
				canQuarantine = false
				return canQuarantine, allowOtherHosts, isLocalAdmin
			}

			if !isLocalAdmin {
				canQuarantine = false
				return canQuarantine, allowOtherHosts, isLocalAdmin
			}

			// They have local admin status and we allow local admins to quarantine
			canQuarantine = true
		}
	}

	return canQuarantine, allowOtherHosts, isLocalAdmin
}
