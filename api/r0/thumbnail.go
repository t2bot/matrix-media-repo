package r0

import (
	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/api/_routers"
	"github.com/turt2live/matrix-media-repo/util"

	"net/http"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/thumbnail_controller"
	"github.com/turt2live/matrix-media-repo/controllers/download_controller"
)

func ThumbnailMedia(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
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
			return _responses.BadRequest("allow_remote flag does not appear to be a boolean")
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

	widthStr := r.URL.Query().Get("width")
	heightStr := r.URL.Query().Get("height")
	method := r.URL.Query().Get("method")
	animatedStr := r.URL.Query().Get("animated")
	if animatedStr == "" {
		animatedStr = r.URL.Query().Get("org.matrix.msc2705.animated")
	}

	if widthStr == "" || heightStr == "" {
		return _responses.BadRequest("Width and height are required")
	}

	width := 0
	height := 0
	animated := rctx.Config.Thumbnails.AllowAnimated && rctx.Config.Thumbnails.DefaultAnimated

	if widthStr != "" {
		parsedWidth, err := strconv.Atoi(widthStr)
		if err != nil {
			return _responses.BadRequest("Width does not appear to be an integer")
		}
		width = parsedWidth
	}
	if heightStr != "" {
		parsedHeight, err := strconv.Atoi(heightStr)
		if err != nil {
			return _responses.BadRequest("Height does not appear to be an integer")
		}
		height = parsedHeight
	}
	if animatedStr != "" {
		parsedFlag, err := strconv.ParseBool(animatedStr)
		if err != nil {
			return _responses.BadRequest("Animated flag does not appear to be a boolean")
		}
		animated = parsedFlag
	}
	if method == "" {
		method = "scale"
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"requestedWidth":    width,
		"requestedHeight":   height,
		"requestedMethod":   method,
		"requestedAnimated": animated,
	})

	if width <= 0 || height <= 0 {
		return _responses.BadRequest("Width and height must be greater than zero")
	}

	streamedThumbnail, err := thumbnail_controller.GetThumbnail(server, mediaId, width, height, animated, method, downloadRemote, rctx)
	if err != nil {
		if err == common.ErrMediaNotFound {
			return _responses.NotFoundError()
		} else if err == common.ErrMediaTooLarge {
			return _responses.RequestTooLarge()
		}
		rctx.Log.Error("Unexpected error locating media: " + err.Error())
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected Error")
	}

	if method == "scale" {
		// now we fetch the original file to check if that one is smaller
		streamedMedia, err := download_controller.GetMedia(server, mediaId, downloadRemote, false, rctx)
		if err == nil && streamedThumbnail.Thumbnail.SizeBytes > streamedMedia.SizeBytes {
			// the media is smaller than the thumbnail, return that instead
			return &DownloadMediaResponse{
				ContentType:       streamedMedia.ContentType,
				Filename:          streamedMedia.UploadName,
				SizeBytes:         streamedMedia.SizeBytes,
				Data:              streamedMedia.Stream,
			}
		}
	}

	return &DownloadMediaResponse{
		ContentType: streamedThumbnail.Thumbnail.ContentType,
		SizeBytes:   streamedThumbnail.Thumbnail.SizeBytes,
		Data:        streamedThumbnail.Stream,
		Filename:    "thumbnail.png",
	}
}
