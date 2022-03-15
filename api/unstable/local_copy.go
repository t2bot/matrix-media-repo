package unstable

import (
	"github.com/getsentry/sentry-go"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/api/r0"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/download_controller"
	"github.com/turt2live/matrix-media-repo/controllers/upload_controller"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
)

func LocalCopy(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	params := mux.Vars(r)

	server := params["server"]
	mediaId := params["mediaId"]
	allowRemote := r.URL.Query().Get("allow_remote")

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
		"allowRemote": downloadRemote,
	})

	// TODO: There's a lot of room for improvement here. Instead of re-uploading media, we should just update the DB.

	streamedMedia, err := download_controller.GetMedia(server, mediaId, downloadRemote, true, nil, rctx)
	if err != nil {
		if err == common.ErrMediaNotFound {
			return api.NotFoundError()
		} else if err == common.ErrMediaTooLarge {
			return api.RequestTooLarge()
		} else if err == common.ErrMediaQuarantined {
			return api.NotFoundError() // We lie for security
		}
		rctx.Log.Error("Unexpected error locating media: " + err.Error())
		sentry.CaptureException(err)
		return api.InternalServerError("Unexpected Error")
	}
	defer cleanup.DumpAndCloseStream(streamedMedia.Stream)

	// Don't clone the media if it's already available on this domain
	if streamedMedia.KnownMedia.Origin == r.Host {
		return &r0.MediaUploadedResponse{ContentUri: streamedMedia.KnownMedia.MxcUri()}
	}

	newMedia, err := upload_controller.UploadMedia(streamedMedia.Stream, streamedMedia.KnownMedia.SizeBytes, streamedMedia.KnownMedia.ContentType, streamedMedia.KnownMedia.UploadName, user.UserId, r.Host, "", rctx)
	if err != nil {
		rctx.Log.Error("Unexpected error storing media: " + err.Error())
		sentry.CaptureException(err)
		return api.InternalServerError("Unexpected Error")
	}

	return &r0.MediaUploadedResponse{ContentUri: newMedia.MxcUri()}
}
