package custom

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/api/apimeta"
	"github.com/t2bot/matrix-media-repo/api/responses"
	"github.com/t2bot/matrix-media-repo/api/routers"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/matrix"
	"github.com/t2bot/matrix-media-repo/util"
)

type Attributes struct {
	Purpose database.Purpose `json:"purpose"`
}

func canChangeAttributes(rctx rcontext.RequestContext, r *http.Request, origin string, user apimeta.UserInfo) bool {
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

func GetAttributes(r *http.Request, rctx rcontext.RequestContext, user apimeta.UserInfo) interface{} {
	origin := routers.GetParam("server", r)
	mediaId := routers.GetParam("mediaId", r)

	if !routers.ServerNameRegex.MatchString(origin) {
		return responses.BadRequest(errors.New("invalid origin"))
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"origin":  origin,
		"mediaId": mediaId,
	})

	if !canChangeAttributes(rctx, r, origin, user) {
		return responses.AuthFailed()
	}

	// Check to see if the media exists
	mediaDb := database.GetInstance().Media.Prepare(rctx)
	media, err := mediaDb.GetById(origin, mediaId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return responses.InternalServerError(errors.New("failed to get media record"))
	}
	if media == nil {
		return responses.NotFoundError()
	}

	attrDb := database.GetInstance().MediaAttributes.Prepare(rctx)
	attrs, err := attrDb.Get(origin, mediaId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return responses.InternalServerError(errors.New("failed to get attributes record"))
	}
	retAttrs := &Attributes{
		Purpose: database.PurposeNone,
	}
	if attrs != nil {
		retAttrs.Purpose = attrs.Purpose
	}

	return &responses.DoNotCacheResponse{Payload: retAttrs}
}

func SetAttributes(r *http.Request, rctx rcontext.RequestContext, user apimeta.UserInfo) interface{} {
	origin := routers.GetParam("server", r)
	mediaId := routers.GetParam("mediaId", r)

	if !routers.ServerNameRegex.MatchString(origin) {
		return responses.BadRequest(errors.New("invalid origin"))
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"origin":  origin,
		"mediaId": mediaId,
	})

	if !canChangeAttributes(rctx, r, origin, user) {
		return responses.AuthFailed()
	}

	defer r.Body.Close()
	newAttrs := &Attributes{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&newAttrs)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return responses.InternalServerError(errors.New("failed to read attributes"))
	}

	attrDb := database.GetInstance().MediaAttributes.Prepare(rctx)
	attrs, err := attrDb.Get(origin, mediaId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return responses.InternalServerError(errors.New("failed to get attributes"))
	}

	if attrs == nil || attrs.Purpose != newAttrs.Purpose {
		if !database.IsPurpose(newAttrs.Purpose) {
			return responses.BadRequest(errors.New("unknown purpose"))
		}
		err = attrDb.UpsertPurpose(origin, mediaId, newAttrs.Purpose)
		if err != nil {
			rctx.Log.Error(err)
			sentry.CaptureException(err)
			return responses.InternalServerError(errors.New("failed to update attributes: purpose"))
		}
	}

	return &responses.DoNotCacheResponse{Payload: newAttrs}
}
