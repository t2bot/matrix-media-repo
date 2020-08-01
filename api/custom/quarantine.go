package custom

import (
	"database/sql"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/internal_cache"
	"github.com/turt2live/matrix-media-repo/matrix"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type MediaQuarantinedResponse struct {
	NumQuarantined int `json:"num_quarantined"`
}

// Developer note: This isn't broken out into a dedicated controller class because the logic is slightly
// too complex to do so. If anything, the logic should be improved and moved.

func QuarantineRoomMedia(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	canQuarantine, allowOtherHosts, isLocalAdmin := getQuarantineRequestInfo(r, rctx, user)
	if !canQuarantine {
		return api.AuthFailed()
	}

	params := mux.Vars(r)

	roomId := params["roomId"]

	rctx = rctx.LogWithFields(logrus.Fields{
		"roomId":     roomId,
		"localAdmin": isLocalAdmin,
	})

	allMedia, err := matrix.ListMedia(rctx, r.Host, user.AccessToken, roomId, r.RemoteAddr)
	if err != nil {
		rctx.Log.Error("Error while listing media in the room: " + err.Error())
		return api.InternalServerError("error retrieving media in room")
	}

	var mxcs []string
	mxcs = append(mxcs, allMedia.LocalMxcs...)
	mxcs = append(mxcs, allMedia.RemoteMxcs...)

	total := 0
	for _, mxc := range mxcs {
		server, mediaId, err := util.SplitMxc(mxc)
		if err != nil {
			rctx.Log.Error("Error parsing MXC URI (" + mxc + "): " + err.Error())
			return api.InternalServerError("error parsing mxc uri")
		}

		if !allowOtherHosts && r.Host != server {
			rctx.Log.Warn("Skipping media " + mxc + " because it is on a different host")
			continue
		}

		resp, ok := doQuarantine(rctx, server, mediaId, allowOtherHosts)
		if !ok {
			return resp
		}

		total += resp.(*MediaQuarantinedResponse).NumQuarantined
	}

	return &api.DoNotCacheResponse{Payload: &MediaQuarantinedResponse{NumQuarantined: total}}
}

func QuarantineUserMedia(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	canQuarantine, allowOtherHosts, isLocalAdmin := getQuarantineRequestInfo(r, rctx, user)
	if !canQuarantine {
		return api.AuthFailed()
	}

	params := mux.Vars(r)

	userId := params["userId"]

	rctx = rctx.LogWithFields(logrus.Fields{
		"userId":     userId,
		"localAdmin": isLocalAdmin,
	})

	_, userDomain, err := util.SplitUserId(userId)
	if err != nil {
		rctx.Log.Error("Error parsing user ID (" + userId + "): " + err.Error())
		return api.InternalServerError("error parsing user ID")
	}

	if !allowOtherHosts && userDomain != r.Host {
		return api.AuthFailed()
	}

	db := storage.GetDatabase().GetMediaStore(rctx)
	userMedia, err := db.GetMediaByUser(userId)
	if err != nil {
		rctx.Log.Error("Error while listing media for the user: " + err.Error())
		return api.InternalServerError("error retrieving media for user")
	}

	total := 0
	for _, media := range userMedia {
		resp, ok := doQuarantineOn(media, allowOtherHosts, rctx)
		if !ok {
			return resp
		}

		total += resp.(*MediaQuarantinedResponse).NumQuarantined
	}

	return &api.DoNotCacheResponse{Payload: &MediaQuarantinedResponse{NumQuarantined: total}}
}

func QuarantineDomainMedia(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	canQuarantine, allowOtherHosts, isLocalAdmin := getQuarantineRequestInfo(r, rctx, user)
	if !canQuarantine {
		return api.AuthFailed()
	}

	params := mux.Vars(r)

	serverName := params["serverName"]

	rctx = rctx.LogWithFields(logrus.Fields{
		"serverName": serverName,
		"localAdmin": isLocalAdmin,
	})

	if !allowOtherHosts && serverName != r.Host {
		return api.AuthFailed()
	}

	db := storage.GetDatabase().GetMediaStore(rctx)
	userMedia, err := db.GetAllMediaForServer(serverName)
	if err != nil {
		rctx.Log.Error("Error while listing media for the server: " + err.Error())
		return api.InternalServerError("error retrieving media for server")
	}

	total := 0
	for _, media := range userMedia {
		resp, ok := doQuarantineOn(media, allowOtherHosts, rctx)
		if !ok {
			return resp
		}

		total += resp.(*MediaQuarantinedResponse).NumQuarantined
	}

	return &api.DoNotCacheResponse{Payload: &MediaQuarantinedResponse{NumQuarantined: total}}
}

func QuarantineMedia(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	canQuarantine, allowOtherHosts, isLocalAdmin := getQuarantineRequestInfo(r, rctx, user)
	if !canQuarantine {
		return api.AuthFailed()
	}

	params := mux.Vars(r)

	server := params["server"]
	mediaId := params["mediaId"]

	rctx = rctx.LogWithFields(logrus.Fields{
		"server":     server,
		"mediaId":    mediaId,
		"localAdmin": isLocalAdmin,
	})

	if !allowOtherHosts && r.Host != server {
		return api.BadRequest("unable to quarantine media on other homeservers")
	}

	resp, _ := doQuarantine(rctx, server, mediaId, allowOtherHosts)
	return &api.DoNotCacheResponse{Payload: resp}
}

func doQuarantine(ctx rcontext.RequestContext, origin string, mediaId string, allowOtherHosts bool) (interface{}, bool) {
	db := storage.GetDatabase().GetMediaStore(ctx)
	media, err := db.Get(origin, mediaId)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.Log.Warn("Media not found, could not quarantine: " + origin + "/" + mediaId)
			return &MediaQuarantinedResponse{0}, true
		}

		ctx.Log.Error("Error fetching media: " + err.Error())
		return api.InternalServerError("error quarantining media"), false
	}

	return doQuarantineOn(media, allowOtherHosts, ctx)
}

func doQuarantineOn(media *types.Media, allowOtherHosts bool, ctx rcontext.RequestContext) (interface{}, bool) {
	// Check to make sure the media doesn't have a purpose in staying
	attrDb := storage.GetDatabase().GetMediaAttributesStore(ctx)
	attr, err := attrDb.GetAttributesDefaulted(media.Origin, media.MediaId)
	if err != nil {
		ctx.Log.Error("Error while getting attributes for media: " + err.Error())
		return api.InternalServerError("Error quarantining media"), false
	}
	if attr.Purpose == types.PurposePinned {
		ctx.Log.Warn("Refusing to quarantine media due to it being pinned")
		return &MediaQuarantinedResponse{NumQuarantined: 0}, true
	}

	// We reset the entire cache to avoid any lingering links floating around, such as thumbnails or other media.
	// The reset is done before actually quarantining the media because that could fail for some reason
	internal_cache.Get().Reset()

	num, err := setMediaQuarantined(media, true, allowOtherHosts, ctx)
	if err != nil {
		ctx.Log.Error("Error quarantining media: " + err.Error())
		return api.InternalServerError("Error quarantining media"), false
	}

	return &MediaQuarantinedResponse{NumQuarantined: num}, true
}

func setMediaQuarantined(media *types.Media, isQuarantined bool, allowOtherHosts bool, ctx rcontext.RequestContext) (int, error) {
	db := storage.GetDatabase().GetMediaStore(ctx)
	numQuarantined := 0

	// Quarantine all media with the same hash, including the one requested
	otherMedia, err := db.GetByHash(media.Sha256Hash)
	if err != nil {
		return numQuarantined, err
	}
	for _, m := range otherMedia {
		if m.Origin != media.Origin && !allowOtherHosts {
			ctx.Log.Warn("Skipping quarantine on " + m.Origin + "/" + m.MediaId + " because it is on a different host from " + media.Origin + "/" + media.MediaId)
			continue
		}

		err := db.SetQuarantined(m.Origin, m.MediaId, isQuarantined)
		if err != nil {
			return numQuarantined, err
		}

		numQuarantined++
		ctx.Log.Warn("Media has been quarantined: " + m.Origin + "/" + m.MediaId)
	}

	return numQuarantined, nil
}

func getQuarantineRequestInfo(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) (bool, bool, bool) {
	isGlobalAdmin := util.IsGlobalAdmin(user.UserId) || user.IsShared
	canQuarantine := isGlobalAdmin
	allowOtherHosts := isGlobalAdmin
	isLocalAdmin := false
	var err error
	if !isGlobalAdmin {
		if rctx.Config.Quarantine.AllowLocalAdmins {
			isLocalAdmin, err = matrix.IsUserAdmin(rctx, r.Host, user.AccessToken, r.RemoteAddr)
			if err != nil {
				rctx.Log.Error("Error verifying local admin: " + err.Error())
				canQuarantine = false
				return canQuarantine, allowOtherHosts, isLocalAdmin
			}

			if !isLocalAdmin {
				rctx.Log.Warn(user.UserId + " tried to quarantine media on another server")
				canQuarantine = false
				return canQuarantine, allowOtherHosts, isLocalAdmin
			}

			// They have local admin status and we allow local admins to quarantine
			canQuarantine = true
		}
	}

	if !canQuarantine {
		rctx.Log.Warn(user.UserId + " tried to quarantine media")
	}

	return canQuarantine, allowOtherHosts, isLocalAdmin
}
