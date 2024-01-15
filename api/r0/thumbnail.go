package r0

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/api/_routers"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/datastores"
	"github.com/turt2live/matrix-media-repo/pipelines/pipeline_download"
	"github.com/turt2live/matrix-media-repo/pipelines/pipeline_thumbnail"
	"github.com/turt2live/matrix-media-repo/util"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

func ThumbnailMedia(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	server := _routers.GetParam("server", r)
	mediaId := _routers.GetParam("mediaId", r)
	allowRemote := r.URL.Query().Get("allow_remote")
	allowRedirect := r.URL.Query().Get("allow_redirect")
	timeoutMs := r.URL.Query().Get("timeout_ms")

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

	canRedirect := false
	if allowRedirect != "" {
		parsedFlag, err := strconv.ParseBool(allowRedirect)
		if err != nil {
			return _responses.BadRequest("allow_redirect flag does not appear to be a boolean")
		}
		canRedirect = parsedFlag
	}

	blockFor, err := util.CalcBlockForDuration(timeoutMs)
	if err != nil {
		return _responses.BadRequest("timeout_ms does not appear to be an integer")
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"mediaId":       mediaId,
		"server":        server,
		"allowRemote":   downloadRemote,
		"allowRedirect": canRedirect,
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

	thumbnail, stream, err := pipeline_thumbnail.Execute(rctx, server, mediaId, pipeline_thumbnail.ThumbnailOpts{
		DownloadOpts: pipeline_download.DownloadOpts{
			FetchRemoteIfNeeded: downloadRemote,
			BlockForReadUntil:   blockFor,
			RecordOnly:          false, // overridden
			CanRedirect:         canRedirect,
		},
		Width:    width,
		Height:   height,
		Method:   method,
		Animated: animated,
	})
	if err != nil {
		var redirect datastores.RedirectError
		if errors.Is(err, common.ErrMediaNotFound) {
			return _responses.NotFoundError()
		} else if errors.Is(err, common.ErrMediaTooLarge) {
			return _responses.RequestTooLarge()
		} else if errors.Is(err, common.ErrMediaQuarantined) {
			rctx.Log.Debug("Quarantined media accessed. Has stream? ", stream != nil)
			if stream != nil {
				return _responses.MakeQuarantinedImageResponse(stream)
			} else {
				return _responses.NotFoundError() // We lie for security
			}
		} else if errors.Is(err, common.ErrMediaNotYetUploaded) {
			return _responses.NotYetUploaded()
		} else if errors.Is(err, common.ErrMediaDimensionsTooSmall) {
			if stream == nil {
				return _responses.NotFoundError() // something went wrong so just 404 the thumbnail
			}

			// We have a stream, and an error about image size, so we know there should be a media record
			mediaDb := database.GetInstance().Media.Prepare(rctx)
			record, err := mediaDb.GetById(server, mediaId)
			if err != nil {
				rctx.Log.Error("Unexpected error locating media record: ", err)
				sentry.CaptureException(err)
				return _responses.InternalServerError("Unexpected Error")
			} else {
				return &_responses.DownloadResponse{
					ContentType:       record.ContentType,
					Filename:          record.UploadName,
					SizeBytes:         record.SizeBytes,
					Data:              stream,
					TargetDisposition: "infer",
				}
			}
		} else if errors.As(err, &redirect) {
			return _responses.Redirect(redirect.RedirectUrl)
		}
		rctx.Log.Error("Unexpected error locating media: ", err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected Error")
	}

	return &_responses.DownloadResponse{
		ContentType:       thumbnail.ContentType,
		Filename:          "thumbnail" + util.ExtensionForContentType(thumbnail.ContentType),
		SizeBytes:         thumbnail.SizeBytes,
		Data:              stream,
		TargetDisposition: "infer",
	}
}
