package r0

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/thumbnail_controller"
)

func ThumbnailMedia(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	params := mux.Vars(r)

	server := params["server"]
	mediaId := params["mediaId"]
	allowRemote := r.URL.Query().Get("allow_remote")

	downloadRemote := true
	if allowRemote != "" {
		parsedFlag, err := strconv.ParseBool(allowRemote)
		if err != nil {
			return api.BadRequest("allow_remote flag does not appear to be a boolean")
		}
		downloadRemote = parsedFlag
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"mediaId":     mediaId,
		"server":      server,
		"allowRemote": downloadRemote,
	})

	widthStr := r.URL.Query().Get("width")
	heightStr := r.URL.Query().Get("height")
	method := r.URL.Query().Get("method")
	animatedStr := r.URL.Query().Get("animated")

	if widthStr == "" || heightStr == "" {
		return api.BadRequest("Width and height are required")
	}

	width := 0
	height := 0
	animated := rctx.Config.Thumbnails.AllowAnimated && rctx.Config.Thumbnails.DefaultAnimated

	if widthStr != "" {
		parsedWidth, err := strconv.Atoi(widthStr)
		if err != nil {
			return api.BadRequest("Width does not appear to be an integer")
		}
		width = parsedWidth
	}
	if heightStr != "" {
		parsedHeight, err := strconv.Atoi(heightStr)
		if err != nil {
			return api.BadRequest("Height does not appear to be an integer")
		}
		height = parsedHeight
	}
	if animatedStr != "" {
		parsedFlag, err := strconv.ParseBool(animatedStr)
		if err != nil {
			return api.BadRequest("Animated flag does not appear to be a boolean")
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
		return api.BadRequest("Width and height must be greater than zero")
	}

	streamedThumbnail, err := thumbnail_controller.GetThumbnail(server, mediaId, width, height, animated, method, downloadRemote, rctx)
	if err != nil {
		if err == common.ErrMediaNotFound {
			return api.NotFoundError()
		} else if err == common.ErrMediaTooLarge {
			return api.RequestTooLarge()
		}
		rctx.Log.Error("Unexpected error locating media: " + err.Error())
		return api.InternalServerError("Unexpected Error")
	}

	return &DownloadMediaResponse{
		ContentType: streamedThumbnail.Thumbnail.ContentType,
		SizeBytes:   streamedThumbnail.Thumbnail.SizeBytes,
		Data:        streamedThumbnail.Stream,
		Filename:    "thumbnail.png",
	}
}
