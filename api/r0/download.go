package r0

import (
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

	rctx = rctx.LogWithFields(logrus.Fields{
		"mediaId":     mediaId,
		"server":      server,
		"filename":    filename,
		"allowRemote": downloadRemote,
	})

	streamedMedia, err := download_controller.GetMedia(server, mediaId, downloadRemote, false, rctx)
	if err != nil {
		if err == common.ErrMediaNotFound {
			return api.NotFoundError()
		} else if err == common.ErrMediaTooLarge {
			return api.RequestTooLarge()
		} else if err == common.ErrMediaQuarantined {
			return api.NotFoundError() // We lie for security
		}
		rctx.Log.Error("Unexpected error locating media: " + err.Error())
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
