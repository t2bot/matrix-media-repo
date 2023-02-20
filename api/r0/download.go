package r0

import (
	"net/http"
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/api/_routers"
	"github.com/turt2live/matrix-media-repo/util"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/download_controller"
)

type DownloadMediaResponse = _responses.DownloadResponse

func DownloadMedia(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	server := _routers.GetParam("server", r)
	mediaId := _routers.GetParam("mediaId", r)
	filename := _routers.GetParam("filename", r)
	allowRemote := r.URL.Query().Get("allow_remote")

	if !_routers.ServerNameRegex.MatchString(server) {
		return _responses.BadRequest("invalid server ID")
	}

	targetDisposition := r.URL.Query().Get("org.matrix.msc2702.asAttachment")
	if targetDisposition == "true" {
		targetDisposition = "attachment"
	} else if targetDisposition == "false" {
		targetDisposition = "inline"
	} else {
		targetDisposition = "infer"
	}

	downloadRemote := true
	if allowRemote != "" {
		parsedFlag, err := strconv.ParseBool(allowRemote)
		if err != nil {
			return _responses.InternalServerError("allow_remote flag does not appear to be a boolean")
		}
		downloadRemote = parsedFlag
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"mediaId":     mediaId,
		"server":      server,
		"filename":    filename,
		"allowRemote": downloadRemote,
	})

	if !util.IsGlobalAdmin(user.UserId) && util.IsHostIgnored(server) {
		rctx.Log.Warn("Request blocked due to domain being ignored.")
		return _responses.MediaBlocked()
	}

	streamedMedia, err := download_controller.GetMedia(server, mediaId, downloadRemote, false, rctx)
	if err != nil {
		if err == common.ErrMediaNotFound {
			return _responses.NotFoundError()
		} else if err == common.ErrMediaTooLarge {
			return _responses.RequestTooLarge()
		} else if err == common.ErrMediaQuarantined {
			return _responses.NotFoundError() // We lie for security
		}
		rctx.Log.Error("Unexpected error locating media: " + err.Error())
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected Error")
	}

	if filename == "" {
		filename = streamedMedia.UploadName
	}

	return &DownloadMediaResponse{
		ContentType:       streamedMedia.ContentType,
		Filename:          filename,
		SizeBytes:         streamedMedia.SizeBytes,
		Data:              streamedMedia.Stream,
		TargetDisposition: targetDisposition,
	}
}
