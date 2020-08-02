package r0

import (
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/api/features"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/info_controller"
	"github.com/turt2live/matrix-media-repo/controllers/upload_controller"
	"github.com/turt2live/matrix-media-repo/quota"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
)

type MediaUploadedResponse struct {
	ContentUri string `json:"content_uri"`
	Blurhash   string `json:"blurhash,omitempty"`
}

func UploadMedia(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	filename := filepath.Base(r.URL.Query().Get("filename"))
	defer cleanup.DumpAndCloseStream(r.Body)

	rctx = rctx.LogWithFields(logrus.Fields{
		"filename": filename,
	})

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
		return api.InternalServerError("Unexpected Error")
	}
	if !inQuota {
		io.Copy(ioutil.Discard, r.Body) // Ditch the entire request
		return api.QuotaExceeded()
	}

	contentLength := upload_controller.EstimateContentLength(r.ContentLength, r.Header.Get("Content-Length"))

	media, err := upload_controller.UploadMedia(r.Body, contentLength, contentType, filename, user.UserId, r.Host, rctx)
	if err != nil {
		io.Copy(ioutil.Discard, r.Body) // Ditch the entire request

		if err == common.ErrMediaQuarantined {
			return api.BadRequest("This file is not permitted on this server")
		}

		rctx.Log.Error("Unexpected error storing media: " + err.Error())
		return api.InternalServerError("Unexpected Error")
	}

	if rctx.Config.Features.MSC2448Blurhash.Enabled && features.IsRoute(r, features.MSC2448UploadRoute) {
		hash, err := info_controller.GetOrCalculateBlurhash(media, rctx)
		if err != nil {
			rctx.Log.Warn("Failed to calculate blurhash: " + err.Error())
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
