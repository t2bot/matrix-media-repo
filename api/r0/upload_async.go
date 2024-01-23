package r0

import (
	"errors"
	"net/http"
	"path/filepath"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/api/_routers"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/pipelines/pipeline_upload"
)

func UploadMediaAsync(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	server := _routers.GetParam("server", r)
	mediaId := _routers.GetParam("mediaId", r)
	filename := filepath.Base(r.URL.Query().Get("filename"))

	rctx = rctx.LogWithFields(logrus.Fields{
		"mediaId":  mediaId,
		"server":   server,
		"filename": filename,
	})

	if r.Host != server {
		return &_responses.ErrorResponse{
			Code:         common.ErrCodeNotFound,
			Message:      "Upload request is for another domain.",
			InternalCode: common.ErrCodeForbidden,
		}
	}

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream" // binary
	}

	// Early sizing constraints (reject requests which claim to be too large/small)
	if sizeRes := uploadRequestSizeCheck(rctx, r); sizeRes != nil {
		return sizeRes
	}

	// Actually upload
	_, err := pipeline_upload.ExecutePut(rctx, server, mediaId, r.Body, contentType, filename, user.UserId)
	if err != nil {
		if errors.Is(err, common.ErrQuotaExceeded) {
			return _responses.QuotaExceeded()
		} else if errors.Is(err, common.ErrAlreadyUploaded) {
			return &_responses.ErrorResponse{
				Code:         common.ErrCodeCannotOverwrite,
				Message:      "This media has already been uploaded.",
				InternalCode: common.ErrCodeCannotOverwrite,
			}
		} else if errors.Is(err, common.ErrWrongUser) {
			return &_responses.ErrorResponse{
				Code:         common.ErrCodeForbidden,
				Message:      "You do not have permission to upload this media.",
				InternalCode: common.ErrCodeForbidden,
			}
		} else if errors.Is(err, common.ErrExpired) {
			return &_responses.ErrorResponse{
				Code:         common.ErrCodeNotFound,
				Message:      "Media expired or not found.",
				InternalCode: common.ErrCodeNotFound,
			}
		}
		rctx.Log.Error("Unexpected error uploading media: ", err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected Error")
	}

	return &MediaUploadedResponse{
		//ContentUri: util.MxcUri(media.Origin, media.MediaId), // This endpoint doesn't return a URI
	}
}
