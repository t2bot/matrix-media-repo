package custom

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/api/_routers"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/tasks/task_runner"

	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/matrix"
	"github.com/t2bot/matrix-media-repo/util"
)

type MediaPurgedResponse struct {
	NumRemoved int `json:"total_removed"`
}

func PurgeRemoteMedia(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	beforeTsStr := r.URL.Query().Get("before_ts")
	if beforeTsStr == "" {
		return _responses.BadRequest(errors.New("Missing before_ts argument"))
	}
	beforeTs, err := strconv.ParseInt(beforeTsStr, 10, 64)
	if err != nil {
		return _responses.BadRequest(fmt.Errorf("Error parsing before_ts: %w", err))
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"beforeTs": beforeTs,
	})

	// We don't bother clearing the cache because it's still probably useful there
	removed, err := task_runner.PurgeRemoteMediaBefore(rctx, beforeTs)
	if err != nil {
		rctx.Log.Error("Error purging remote media: ", err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(errors.New("Error purging remote media"))
	}

	return &_responses.DoNotCacheResponse{Payload: &MediaPurgedResponse{NumRemoved: removed}}
}

func PurgeIndividualRecord(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	authCtx, _, _ := getPurgeAuthContext(rctx, r, user)

	server := _routers.GetParam("server", r)
	mediaId := _routers.GetParam("mediaId", r)

	if !_routers.ServerNameRegex.MatchString(server) {
		return _responses.BadRequest(errors.New("invalid server ID"))
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"server":  server,
		"mediaId": mediaId,
	})

	_, err := task_runner.PurgeMedia(rctx, authCtx, &task_runner.QuarantineThis{
		Single: &task_runner.QuarantineRecord{
			Origin:  server,
			MediaId: mediaId,
		},
	})
	if err != nil {
		if errors.Is(err, common.ErrWrongUser) {
			return _responses.AuthFailed()
		}
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(errors.New("unexpected error"))
	}

	return &_responses.DoNotCacheResponse{Payload: map[string]interface{}{"purged": true}}
}

func PurgeQuarantined(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	authCtx, isGlobalAdmin, isLocalAdmin := getPurgeAuthContext(rctx, r, user)

	var affected []*database.DbMedia
	var err error

	mediaDb := database.GetInstance().Media.Prepare(rctx)
	if isGlobalAdmin {
		affected, err = mediaDb.GetByQuarantine()
	} else if isLocalAdmin {
		affected, err = mediaDb.GetByOriginQuarantine(r.Host)
	} else {
		return _responses.AuthFailed()
	}
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(errors.New("error fetching media records"))
	}

	mxcs, err := task_runner.PurgeMedia(rctx, authCtx, &task_runner.QuarantineThis{
		DbMedia: affected,
	})
	if err != nil {
		if errors.Is(err, common.ErrWrongUser) {
			return _responses.AuthFailed()
		}
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(errors.New("unexpected error"))
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
			return _responses.BadRequest(fmt.Errorf("Error parsing before_ts: %w", err))
		}
	}

	includeLocal := false
	includeLocalStr := r.URL.Query().Get("include_local")
	if includeLocalStr != "" {
		includeLocal, err = strconv.ParseBool(includeLocalStr)
		if err != nil {
			return _responses.BadRequest(fmt.Errorf("Error parsing include_local: %w", err))
		}
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"before_ts":     beforeTs,
		"include_local": includeLocal,
	})

	domains := make([]string, 0)
	if !includeLocal {
		domains = util.GetOurDomains()
	}

	mediaDb := database.GetInstance().Media.Prepare(rctx)
	records, err := mediaDb.GetOldExcluding(domains, beforeTs)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(errors.New("error fetching media records"))
	}

	mxcs, err := task_runner.PurgeMedia(rctx, &task_runner.PurgeAuthContext{}, &task_runner.QuarantineThis{
		DbMedia: records,
	})
	if err != nil {
		if errors.Is(err, common.ErrWrongUser) {
			return _responses.AuthFailed()
		}
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(errors.New("unexpected error"))
	}

	return &_responses.DoNotCacheResponse{Payload: map[string]interface{}{"purged": true, "affected": mxcs}}
}

func PurgeUserMedia(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	authCtx, isGlobalAdmin, isLocalAdmin := getPurgeAuthContext(rctx, r, user)
	if !isGlobalAdmin && !isLocalAdmin {
		return _responses.AuthFailed()
	}

	var err error
	beforeTs := util.NowMillis()
	beforeTsStr := r.URL.Query().Get("before_ts")
	if beforeTsStr != "" {
		beforeTs, err = strconv.ParseInt(beforeTsStr, 10, 64)
		if err != nil {
			return _responses.BadRequest(fmt.Errorf("Error parsing before_ts: %w", err))
		}
	}

	userId := _routers.GetParam("userId", r)

	rctx = rctx.LogWithFields(logrus.Fields{
		"userId":   userId,
		"beforeTs": beforeTs,
	})

	_, userDomain, err := util.SplitUserId(userId)
	if err != nil {
		rctx.Log.Errorf("Error parsing user ID (%s): %v", userId, err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(errors.New("error parsing user ID"))
	}

	if !isGlobalAdmin && userDomain != r.Host {
		return _responses.AuthFailed()
	}

	mediaDb := database.GetInstance().Media.Prepare(rctx)
	records, err := mediaDb.GetOldByUserId(userId, beforeTs)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(errors.New("error fetching media records"))
	}

	mxcs, err := task_runner.PurgeMedia(rctx, authCtx, &task_runner.QuarantineThis{
		DbMedia: records,
	})
	if err != nil {
		if errors.Is(err, common.ErrWrongUser) {
			return _responses.AuthFailed()
		}
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(errors.New("unexpected error"))
	}

	return &_responses.DoNotCacheResponse{Payload: map[string]interface{}{"purged": true, "affected": mxcs}}
}

func PurgeRoomMedia(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	authCtx, isGlobalAdmin, isLocalAdmin := getPurgeAuthContext(rctx, r, user)
	if !isGlobalAdmin && !isLocalAdmin {
		return _responses.AuthFailed()
	}

	var err error
	beforeTs := util.NowMillis()
	beforeTsStr := r.URL.Query().Get("before_ts")
	if beforeTsStr != "" {
		beforeTs, err = strconv.ParseInt(beforeTsStr, 10, 64)
		if err != nil {
			return _responses.BadRequest(fmt.Errorf("Error parsing before_ts: %w", err))
		}
	}

	roomId := _routers.GetParam("roomId", r)

	rctx = rctx.LogWithFields(logrus.Fields{
		"roomId":   roomId,
		"beforeTs": beforeTs,
	})

	allMedia, err := matrix.ListMedia(rctx, r.Host, user.AccessToken, roomId, r.RemoteAddr)
	if err != nil {
		rctx.Log.Error("Error while listing media in the room: ", err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(errors.New("error retrieving media in room"))
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
		mxcs = append(mxcs, allMedia.LocalMxcs...)
		mxcs = append(mxcs, allMedia.RemoteMxcs...)
	}

	mxcs2, err := task_runner.PurgeMedia(rctx, authCtx, &task_runner.QuarantineThis{
		MxcUris: mxcs,
	})
	if err != nil {
		if errors.Is(err, common.ErrWrongUser) {
			return _responses.AuthFailed()
		}
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(errors.New("unexpected error"))
	}

	return &_responses.DoNotCacheResponse{Payload: map[string]interface{}{"purged": true, "affected": mxcs2}}
}

func PurgeDomainMedia(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	authCtx, isGlobalAdmin, isLocalAdmin := getPurgeAuthContext(rctx, r, user)
	if !isGlobalAdmin && !isLocalAdmin {
		return _responses.AuthFailed()
	}

	var err error
	beforeTs := util.NowMillis()
	beforeTsStr := r.URL.Query().Get("before_ts")
	if beforeTsStr != "" {
		beforeTs, err = strconv.ParseInt(beforeTsStr, 10, 64)
		if err != nil {
			return _responses.BadRequest(fmt.Errorf("Error parsing before_ts: %f", err))
		}
	}

	serverName := _routers.GetParam("serverName", r)

	if !_routers.ServerNameRegex.MatchString(serverName) {
		return _responses.BadRequest(errors.New("invalid server name"))
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"serverName": serverName,
		"beforeTs":   beforeTs,
	})

	if !isGlobalAdmin && serverName != r.Host {
		return _responses.AuthFailed()
	}

	mediaDb := database.GetInstance().Media.Prepare(rctx)
	records, err := mediaDb.GetOldByOrigin(serverName, beforeTs)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(errors.New("error fetching media records"))
	}

	mxcs, err := task_runner.PurgeMedia(rctx, authCtx, &task_runner.QuarantineThis{
		DbMedia: records,
	})
	if err != nil {
		if errors.Is(err, common.ErrWrongUser) {
			return _responses.AuthFailed()
		}
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(errors.New("unexpected error"))
	}

	return &_responses.DoNotCacheResponse{Payload: map[string]interface{}{"purged": true, "affected": mxcs}}
}

func getPurgeAuthContext(ctx rcontext.RequestContext, r *http.Request, user _apimeta.UserInfo) (*task_runner.PurgeAuthContext, bool, bool) {
	globalAdmin, localAdmin := _apimeta.GetRequestUserAdminStatus(r, ctx, user)
	if globalAdmin {
		return &task_runner.PurgeAuthContext{}, true, localAdmin
	}
	if localAdmin {
		return &task_runner.PurgeAuthContext{SourceOrigin: r.Host}, false, true
	}
	return &task_runner.PurgeAuthContext{UploaderUserId: user.UserId}, false, false
}
