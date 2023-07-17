package custom

import (
	"encoding/json"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/api/_routers"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/matrix"
	"github.com/turt2live/matrix-media-repo/util"
)

type Attributes struct {
	Purpose database.Purpose `json:"purpose"`
}

func canChangeAttributes(rctx rcontext.RequestContext, r *http.Request, origin string, user _apimeta.UserInfo) bool {
	isGlobalAdmin := util.IsGlobalAdmin(user.UserId) || user.IsShared
	if isGlobalAdmin {
		return true
	}

	isLocalAdmin, err := matrix.IsUserAdmin(rctx, origin, user.AccessToken, r.RemoteAddr)
	if err != nil {
		return false
	}
	return isLocalAdmin
}

func GetAttributes(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	origin := _routers.GetParam("server", r)
	mediaId := _routers.GetParam("mediaId", r)

	if !_routers.ServerNameRegex.MatchString(origin) {
		return _responses.BadRequest("invalid origin")
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"origin":  origin,
		"mediaId": mediaId,
	})

	if !canChangeAttributes(rctx, r, origin, user) {
		return _responses.AuthFailed()
	}

	// Check to see if the media exists
	mediaDb := database.GetInstance().Media.Prepare(rctx)
	media, err := mediaDb.GetById(origin, mediaId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to get media record")
	}
	if media == nil {
		return _responses.NotFoundError()
	}

	attrDb := database.GetInstance().MediaAttributes.Prepare(rctx)
	attrs, err := attrDb.Get(origin, mediaId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to get attributes record")
	}
	retAttrs := &Attributes{
		Purpose: database.PurposeNone,
	}
	if attrs != nil {
		retAttrs.Purpose = attrs.Purpose
	}

	return &_responses.DoNotCacheResponse{Payload: retAttrs}
}

func SetAttributes(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	origin := _routers.GetParam("server", r)
	mediaId := _routers.GetParam("mediaId", r)

	if !_routers.ServerNameRegex.MatchString(origin) {
		return _responses.BadRequest("invalid origin")
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"origin":  origin,
		"mediaId": mediaId,
	})

	if !canChangeAttributes(rctx, r, origin, user) {
		return _responses.AuthFailed()
	}

	defer r.Body.Close()
	newAttrs := &Attributes{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&newAttrs)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to read attributes")
	}

	attrDb := database.GetInstance().MediaAttributes.Prepare(rctx)
	attrs, err := attrDb.Get(origin, mediaId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to get attributes")
	}

	if attrs == nil || attrs.Purpose != newAttrs.Purpose {
		if !database.IsPurpose(newAttrs.Purpose) {
			return _responses.BadRequest("unknown purpose")
		}
		err = attrDb.UpsertPurpose(origin, mediaId, newAttrs.Purpose)
		if err != nil {
			rctx.Log.Error(err)
			sentry.CaptureException(err)
			return _responses.InternalServerError("failed to update attributes: purpose")
		}
	}

	return &_responses.DoNotCacheResponse{Payload: newAttrs}
}
