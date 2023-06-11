package r0

import (
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/datastores"
	"github.com/turt2live/matrix-media-repo/pipelines/pipeline_upload"
	"github.com/turt2live/matrix-media-repo/util"
)

type MediaUploadedResponse struct {
	ContentUri string `json:"content_uri,omitempty"`
	Blurhash   string `json:"xyz.amorgan.blurhash,omitempty"`
}

func UploadMedia(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	filename := filepath.Base(r.URL.Query().Get("filename"))

	rctx = rctx.LogWithFields(logrus.Fields{
		"filename": filename,
	})

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream" // binary
	}

	// Early sizing constraints (reject requests which claim to be too large/small)
	if sizeRes := uploadRequestSizeCheck(rctx, r); sizeRes != nil {
		return sizeRes
	}

	// Actually upload
	media, err := pipeline_upload.Execute(rctx, r.Host, "", r.Body, contentType, filename, user.UserId, datastores.LocalMediaKind)
	if err != nil {
		if err == common.ErrQuotaExceeded {
			return _responses.QuotaExceeded()
		}
		rctx.Log.Error("Unexpected error uploading media: " + err.Error())
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected Error")
	}

	blurhash, err := database.GetInstance().Blurhashes.Prepare(rctx).Get(media.Sha256Hash)
	if err != nil {
		rctx.Log.Warn("Unexpected error getting media's blurhash from DB: " + err.Error())
		sentry.CaptureException(err)
	}

	return &MediaUploadedResponse{
		ContentUri: util.MxcUri(media.Origin, media.MediaId),
		Blurhash:   blurhash,
	}
}

func uploadRequestSizeCheck(rctx rcontext.RequestContext, r *http.Request) *_responses.ErrorResponse {
	maxSize := rctx.Config.Uploads.MaxSizeBytes
	minSize := rctx.Config.Uploads.MinSizeBytes
	if maxSize > 0 || minSize > 0 {
		if r.ContentLength > 0 {
			if maxSize > 0 && maxSize < r.ContentLength {
				return _responses.RequestTooLarge()
			}
			if minSize > 0 && minSize > r.ContentLength {
				return _responses.RequestTooSmall()
			}
		} else {
			header := r.Header.Get("Content-Length")
			if header != "" {
				parsed, _ := strconv.ParseInt(header, 10, 64)
				if maxSize > 0 && maxSize < parsed {
					return _responses.RequestTooLarge()
				}
				if minSize > 0 && minSize > parsed {
					return _responses.RequestTooSmall()
				}
			}
		}
	}
	return nil
}
