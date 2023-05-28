package r0

import (
	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/datastores"
	"github.com/turt2live/matrix-media-repo/pipline/upload_pipeline"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/stream_util"

	"io"
	"net/http"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/upload_controller"
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

	// TODO: Move to new function - https://github.com/turt2live/matrix-media-repo/issues/411
	if upload_controller.IsRequestTooLarge(r.ContentLength, r.Header.Get("Content-Length"), rctx) {
		io.Copy(io.Discard, r.Body) // Ditch the entire request
		return _responses.RequestTooLarge()
	}

	// TODO: Move to new function - https://github.com/turt2live/matrix-media-repo/issues/411
	if upload_controller.IsRequestTooSmall(r.ContentLength, r.Header.Get("Content-Length"), rctx) {
		io.Copy(io.Discard, r.Body) // Ditch the entire request
		return _responses.RequestTooSmall()
	}

	media, err := upload_pipeline.UploadMedia(rctx, r.Host, "", r.Body, contentType, filename, user.UserId, datastores.LocalMediaKind)
	if err != nil {
		if err == common.ErrQuotaExceeded {
			return _responses.QuotaExceeded()
		}
		rctx.Log.Error("Unexpected error uploading media: " + err.Error())
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected Error")
	}

	return &MediaUploadedResponse{
		ContentUri: util.MxcUri(media.Origin, media.MediaId),
		//Blurhash:   "", // TODO: Re-add blurhash support - https://github.com/turt2live/matrix-media-repo/issues/411
	}

	//if rctx.Config.Features.MSC2448Blurhash.Enabled && r.URL.Query().Get("xyz.amorgan.generate_blurhash") == "true" {
	//	hash, err := info_controller.GetOrCalculateBlurhash(media, rctx)
	//	if err != nil {
	//		rctx.Log.Warn("Failed to calculate blurhash: " + err.Error())
	//		sentry.CaptureException(err)
	//	}
	//
	//	return &MediaUploadedResponse{
	//		ContentUri: media.MxcUri(),
	//		Blurhash:   hash,
	//	}
	//}
}
