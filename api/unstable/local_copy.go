package unstable

import (
	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/api/_routers"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/stream_util"

	"net/http"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api/r0"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/download_controller"
	"github.com/turt2live/matrix-media-repo/controllers/upload_controller"
)

func LocalCopy(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	server := _routers.GetParam("server", r)
	mediaId := _routers.GetParam("mediaId", r)
	allowRemote := r.URL.Query().Get("allow_remote")

	if !_routers.ServerNameRegex.MatchString(server) {
		return _responses.BadRequest("invalid server ID")
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
		"allowRemote": downloadRemote,
	})

	if !util.IsGlobalAdmin(user.UserId) && util.IsHostIgnored(server) {
		rctx.Log.Warn("Request blocked due to domain being ignored.")
		return _responses.MediaBlocked()
	}

	// TODO: There's a lot of room for improvement here. Instead of re-uploading media, we should just update the DB.

	streamedMedia, err := download_controller.GetMedia(server, mediaId, downloadRemote, true, rctx)
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
	defer stream_util.DumpAndCloseStream(streamedMedia.Stream)

	// Don't clone the media if it's already available on this domain
	if streamedMedia.KnownMedia.Origin == r.Host {
		return &r0.MediaUploadedResponse{ContentUri: streamedMedia.KnownMedia.MxcUri()}
	}

	newMedia, err := upload_controller.UploadMedia(streamedMedia.Stream, streamedMedia.KnownMedia.SizeBytes, streamedMedia.KnownMedia.ContentType, streamedMedia.KnownMedia.UploadName, user.UserId, r.Host, rctx)
	if err != nil {
		rctx.Log.Error("Unexpected error storing media: " + err.Error())
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected Error")
	}

	return &r0.MediaUploadedResponse{ContentUri: newMedia.MxcUri()}
}
