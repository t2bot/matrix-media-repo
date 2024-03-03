package r0

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/api/_routers"
	"github.com/t2bot/matrix-media-repo/api/apimeta"
	"github.com/t2bot/matrix-media-repo/api/responses"
	"github.com/t2bot/matrix-media-repo/datastores"
	"github.com/t2bot/matrix-media-repo/pipelines/pipeline_download"
	"github.com/t2bot/matrix-media-repo/util"

	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

func DownloadMedia(r *http.Request, rctx rcontext.RequestContext, user apimeta.UserInfo) interface{} {
	server := _routers.GetParam("server", r)
	mediaId := _routers.GetParam("mediaId", r)
	filename := _routers.GetParam("filename", r)
	allowRemote := r.URL.Query().Get("allow_remote")
	allowRedirect := r.URL.Query().Get("allow_redirect")
	timeoutMs := r.URL.Query().Get("timeout_ms")

	if !_routers.ServerNameRegex.MatchString(server) {
		return responses.BadRequest(errors.New("invalid server ID"))
	}

	downloadRemote := true
	if allowRemote != "" {
		parsedFlag, err := strconv.ParseBool(allowRemote)
		if err != nil {
			return responses.BadRequest(errors.New("allow_remote flag does not appear to be a boolean"))
		}
		downloadRemote = parsedFlag
	}

	canRedirect := false
	if allowRedirect != "" {
		parsedFlag, err := strconv.ParseBool(allowRedirect)
		if err != nil {
			return responses.BadRequest(errors.New("allow_redirect flag does not appear to be a boolean"))
		}
		canRedirect = parsedFlag
	}

	timeoutMS, err := strconv.ParseInt(timeoutMs, 10, 64)
	if err != nil {
		return responses.BadRequest(errors.New("timeout_ms does not appear to be an integer"))
	}
	timeout := time.Duration(timeoutMS) * time.Millisecond
	if timeout > time.Minute {
		timeout = time.Minute
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"mediaId":       mediaId,
		"server":        server,
		"filename":      filename,
		"allowRemote":   downloadRemote,
		"allowRedirect": canRedirect,
	})

	if !util.IsGlobalAdmin(user.UserId) && util.IsHostIgnored(server) {
		rctx.Log.Warn("Request blocked due to domain being ignored.")
		return responses.MediaBlocked()
	}

	media, stream, err := pipeline_download.Execute(rctx, server, mediaId, pipeline_download.DownloadOpts{
		FetchRemoteIfNeeded: downloadRemote,
		BlockForReadUntil:   timeout,
		CanRedirect:         canRedirect,
	})
	if err != nil {
		var redirect datastores.RedirectError
		if errors.Is(err, common.ErrMediaNotFound) {
			return responses.NotFoundError()
		} else if errors.Is(err, common.ErrMediaTooLarge) {
			return responses.RequestTooLarge()
		} else if errors.Is(err, common.ErrMediaQuarantined) {
			rctx.Log.Debug("Quarantined media accessed. Has stream? ", stream != nil)
			if stream != nil {
				return responses.MakeQuarantinedImageResponse(stream)
			} else {
				return responses.NotFoundError() // We lie for security
			}
		} else if errors.Is(err, common.ErrMediaNotYetUploaded) {
			return responses.NotYetUploaded()
		} else if errors.As(err, &redirect) {
			return responses.Redirect(redirect.RedirectUrl)
		}
		rctx.Log.Error("Unexpected error locating media: ", err)
		sentry.CaptureException(err)
		return responses.InternalServerError(errors.New("Unexpected Error"))
	}

	if filename == "" {
		filename = media.UploadName
	}

	return &responses.DownloadResponse{
		ContentType:       media.ContentType,
		Filename:          filename,
		SizeBytes:         media.SizeBytes,
		Data:              stream,
		TargetDisposition: "infer",
	}
}
