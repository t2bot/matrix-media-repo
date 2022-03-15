package r0

import (
	"github.com/getsentry/sentry-go"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/info_controller"
	"github.com/turt2live/matrix-media-repo/controllers/upload_controller"
	"github.com/turt2live/matrix-media-repo/quota"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
)

type MediaUploadedResponse struct {
	ContentUri string `json:"content_uri"`
	Blurhash   string `json:"xyz.amorgan.blurhash,omitempty"`
}

type MediaCreatedResponse struct {
	ContentUri      string `json:"content_uri"`
	UnusedExpiresAt int64  `json:"unused_expires_at"`
}

func CreateMedia(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	media, _, err := upload_controller.CreateMedia(r.Host, rctx)
	if err != nil {
		rctx.Log.Error("Unexpected error creating media reference: " + err.Error())
		return api.InternalServerError("Unexpected Error")
	}

	if err = upload_controller.PersistMedia(media, user.UserId, rctx); err != nil {
		rctx.Log.Error("Unexpected error persisting media reference: " + err.Error())
		return api.InternalServerError("Unexpected Error")
	}

	return &MediaCreatedResponse{
		ContentUri:      media.MxcUri(),
		UnusedExpiresAt: time.Now().Unix() + int64(rctx.Config.Features.MSC2246Async.AsyncUploadExpirySecs),
	}
}

func UploadMedia(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	var server = ""
	var mediaId = ""

	filename := filepath.Base(r.URL.Query().Get("filename"))
	defer cleanup.DumpAndCloseStream(r.Body)

	if rctx.Config.Features.MSC2246Async.Enabled {
		params := mux.Vars(r)
		server = params["server"]
		mediaId = params["mediaId"]
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"server":   server,
		"mediaId":  mediaId,
		"filename": filename,
	})

	if server != "" && (!util.IsServerOurs(server) || server != r.Host) {
		return api.NotFoundError()
	}

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream" // binary
	}

	if upload_controller.IsRequestTooLarge(r.ContentLength, r.Header.Get("Content-Length"), rctx) {
		io.Copy(ioutil.Discard, r.Body) // Ditch the entire request
		return api.RequestTooLarge()
	}

	if upload_controller.IsRequestTooSmall(r.ContentLength, r.Header.Get("Content-Length"), rctx) {
		io.Copy(ioutil.Discard, r.Body) // Ditch the entire request
		return api.RequestTooSmall()
	}

	inQuota, err := quota.IsUserWithinQuota(rctx, user.UserId)
	if err != nil {
		io.Copy(ioutil.Discard, r.Body) // Ditch the entire request
		rctx.Log.Error("Unexpected error checking quota: " + err.Error())
		sentry.CaptureException(err)
		return api.InternalServerError("Unexpected Error")
	}
	if !inQuota {
		io.Copy(ioutil.Discard, r.Body) // Ditch the entire request
		return api.QuotaExceeded()
	}

	contentLength := upload_controller.EstimateContentLength(r.ContentLength, r.Header.Get("Content-Length"))

	media, err := upload_controller.UploadMedia(r.Body, contentLength, contentType, filename, user.UserId, r.Host, mediaId, rctx)
	if err != nil {
		io.Copy(ioutil.Discard, r.Body) // Ditch the entire request

		if err == common.ErrMediaQuarantined {
			return api.BadRequest("This file is not permitted on this server")
		} else if err == common.ErrCannotOverwriteMedia {
			return api.CannotOverwriteMedia()
		} else if err == common.ErrMediaNotFound {
			return api.NotFoundError()
		}

		rctx.Log.Error("Unexpected error storing media: " + err.Error())
		sentry.CaptureException(err)
		return api.InternalServerError("Unexpected Error")
	}

	if rctx.Config.Features.MSC2448Blurhash.Enabled && r.URL.Query().Get("xyz.amorgan.generate_blurhash") == "true" {
		hash, err := info_controller.GetOrCalculateBlurhash(media, rctx)
		if err != nil {
			rctx.Log.Warn("Failed to calculate blurhash: " + err.Error())
			sentry.CaptureException(err)
		}

		return &MediaUploadedResponse{
			ContentUri: media.MxcUri(),
			Blurhash:   hash,
		}
	}

	return &MediaUploadedResponse{
		ContentUri: media.MxcUri(),
	}
}
