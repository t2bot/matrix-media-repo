package r0

import (
	"errors"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/datastores"
	"github.com/t2bot/matrix-media-repo/pipelines/pipeline_upload"
	"github.com/t2bot/matrix-media-repo/util"
)

type MediaUploadedResponse struct {
	ContentUri string `json:"content_uri,omitempty"`
}

func UploadMediaSync(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
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
		if errors.Is(err, common.ErrQuotaExceeded) {
			return _responses.QuotaExceeded()
		}
		rctx.Log.Error("Unexpected error uploading media: ", err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(errors.New("Unexpected Error"))
	}

	return &MediaUploadedResponse{
		ContentUri: util.MxcUri(media.Origin, media.MediaId),
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
