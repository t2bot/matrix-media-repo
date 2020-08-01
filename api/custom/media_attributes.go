package custom

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/matrix"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
)

type Attributes struct {
	Purpose string `json:"purpose"`
}

func canChangeAttributes(rctx rcontext.RequestContext, r *http.Request, origin string, user api.UserInfo) bool {
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

func GetAttributes(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	params := mux.Vars(r)

	origin := params["server"]
	mediaId := params["mediaId"]

	rctx = rctx.LogWithFields(logrus.Fields{
		"origin":  origin,
		"mediaId": mediaId,
	})

	if !canChangeAttributes(rctx, r, origin, user) {
		return api.AuthFailed()
	}

	db := storage.GetDatabase().GetMediaAttributesStore(rctx)

	attrs, err := db.GetAttributes(origin, mediaId)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("failed to get attributes")
	}

	return &api.DoNotCacheResponse{Payload: &Attributes{
		Purpose: attrs.Purpose,
	}}
}

func SetAttributes(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	params := mux.Vars(r)

	origin := params["server"]
	mediaId := params["mediaId"]

	rctx = rctx.LogWithFields(logrus.Fields{
		"origin":  origin,
		"mediaId": mediaId,
	})

	if !canChangeAttributes(rctx, r, origin, user) {
		return api.AuthFailed()
	}

	defer cleanup.DumpAndCloseStream(r.Body)
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("failed to read attributes")
	}

	newAttrs := &Attributes{}
	err = json.Unmarshal(b, &newAttrs)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("failed to parse attributes")
	}

	db := storage.GetDatabase().GetMediaAttributesStore(rctx)

	attrs, err := db.GetAttributesDefaulted(origin, mediaId)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("failed to get attributes")
	}

	if attrs.Purpose != newAttrs.Purpose {
		if !util.ArrayContains(types.AllPurposes, newAttrs.Purpose) {
			return api.BadRequest("unknown purpose")
		}
		err = db.UpsertPurpose(origin, mediaId, newAttrs.Purpose)
		if err != nil {
			rctx.Log.Error(err)
			return api.InternalServerError("failed to update attributes: purpose")
		}
	}

	return &api.DoNotCacheResponse{Payload: newAttrs}
}
