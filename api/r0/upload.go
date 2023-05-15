package r0

import (
	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/util/stream_util"

	"io"
	"net/http"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/info_controller"
	"github.com/turt2live/matrix-media-repo/controllers/upload_controller"
	"github.com/turt2live/matrix-media-repo/quota"
)

type MediaUploadedResponse struct {
	ContentUri string `json:"content_uri"`
	Blurhash   string `json:"xyz.amorgan.blurhash,omitempty"`
}

func UploadMedia(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	filename := filepath.Base(r.URL.Query().Get("filename"))
	defer stream_util.DumpAndCloseStream(r.Body)

	rctx = rctx.LogWithFields(logrus.Fields{
		"filename": filename,
	})

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream" // binary
	}

	if upload_controller.IsRequestTooLargeForUser(r.ContentLength, r.Header.Get("Content-Length"), rctx, user.UserId) {
		io.Copy(ioutil.Discard, r.Body) // Ditch the entire request
		return api.RequestTooLarge()
	}

	if upload_controller.IsRequestTooLarge(r.ContentLength, r.Header.Get("Content-Length"), rctx) {
		io.Copy(io.Discard, r.Body) // Ditch the entire request
		return _responses.RequestTooLarge()
	}

	if upload_controller.IsRequestTooSmall(r.ContentLength, r.Header.Get("Content-Length"), rctx) {
		io.Copy(io.Discard, r.Body) // Ditch the entire request
		return _responses.RequestTooSmall()
	}

	inQuota, err := quota.IsUserWithinQuota(rctx, user.UserId)
	if err != nil {
		io.Copy(io.Discard, r.Body) // Ditch the entire request
		rctx.Log.Error("Unexpected error checking quota: " + err.Error())
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected Error")
	}
	if !inQuota {
		io.Copy(io.Discard, r.Body) // Ditch the entire request
		return _responses.QuotaExceeded()
	}

	contentLength := upload_controller.EstimateContentLength(r.ContentLength, r.Header.Get("Content-Length"))

	media, err := upload_controller.UploadMedia(r.Body, contentLength, contentType, filename, user.UserId, r.Host, rctx)
	if err != nil {
		io.Copy(io.Discard, r.Body) // Ditch the entire request

		if err == common.ErrMediaQuarantined {
			return _responses.BadRequest("This file is not permitted on this server")
		}

		rctx.Log.Error("Unexpected error storing media: " + err.Error())
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected Error")
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
