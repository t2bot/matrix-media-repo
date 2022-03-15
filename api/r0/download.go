package r0

import (
	"github.com/getsentry/sentry-go"
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/download_controller"
)

type DownloadMediaResponse struct {
	ContentType       string
	Filename          string
	SizeBytes         int64
	Data              io.ReadCloser
	TargetDisposition string
}

func DownloadMedia(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	params := mux.Vars(r)

	server := params["server"]
	mediaId := params["mediaId"]
	filename := params["filename"]
	allowRemote := r.URL.Query().Get("allow_remote")

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
			return api.InternalServerError("allow_remote flag does not appear to be a boolean")
		}
		downloadRemote = parsedFlag
	}

	var asyncWaitMs *int = nil
	if rctx.Config.Features.MSC2246Async.Enabled {
		// request default wait time if feature enabled
		var parsedInt int = -1
		maxStallMs := r.URL.Query().Get("fi.mau.msc2246.max_stall_ms")
		if maxStallMs != "" {
			var err error
			parsedInt, err = strconv.Atoi(maxStallMs)
			if err != nil {
				return api.InternalServerError("fi.mau.msc2246.max_stall_ms does not appear to be a number")
			}
		}
		asyncWaitMs = &parsedInt
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"mediaId":     mediaId,
		"server":      server,
		"filename":    filename,
		"allowRemote": downloadRemote,
	})

	streamedMedia, err := download_controller.GetMedia(server, mediaId, downloadRemote, false, asyncWaitMs, rctx)
	if err != nil {
		if err == common.ErrMediaNotFound {
			return api.NotFoundError()
		} else if err == common.ErrMediaTooLarge {
			return api.RequestTooLarge()
		} else if err == common.ErrMediaQuarantined {
			return api.NotFoundError() // We lie for security
		} else if err == common.ErrNotYetUploaded {
			return api.NotYetUploaded()
		}
		rctx.Log.Error("Unexpected error locating media: " + err.Error())
		sentry.CaptureException(err)
		return api.InternalServerError("Unexpected Error")
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
