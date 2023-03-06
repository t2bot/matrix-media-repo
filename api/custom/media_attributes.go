package custom

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/api/_routers"
	"github.com/turt2live/matrix-media-repo/util/stream_util"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/matrix"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type Attributes struct {
	Purpose string `json:"purpose"`
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
	mediaDb := storage.GetDatabase().GetMediaStore(rctx)
	media, err := mediaDb.Get(origin, mediaId)
	if err != nil && err != sql.ErrNoRows {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to get media record")
	}
	if media == nil || err == sql.ErrNoRows {
		return _responses.NotFoundError()
	}

	db := storage.GetDatabase().GetMediaAttributesStore(rctx)

	attrs, err := db.GetAttributesDefaulted(origin, mediaId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to get attributes")
	}

	return &_responses.DoNotCacheResponse{Payload: &Attributes{
		Purpose: attrs.Purpose,
	}}
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

	defer stream_util.DumpAndCloseStream(r.Body)
	b, err := io.ReadAll(r.Body)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to read attributes")
	}

	newAttrs := &Attributes{}
	err = json.Unmarshal(b, &newAttrs)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to parse attributes")
	}

	db := storage.GetDatabase().GetMediaAttributesStore(rctx)

	attrs, err := db.GetAttributesDefaulted(origin, mediaId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to get attributes")
	}

	if attrs.Purpose != newAttrs.Purpose {
		if !util.ArrayContains(types.AllPurposes, newAttrs.Purpose) {
			return _responses.BadRequest("unknown purpose")
		}
		err = db.UpsertPurpose(origin, mediaId, newAttrs.Purpose)
		if err != nil {
			rctx.Log.Error(err)
			sentry.CaptureException(err)
			return _responses.InternalServerError("failed to update attributes: purpose")
		}
	}

	return &_responses.DoNotCacheResponse{Payload: newAttrs}
}
