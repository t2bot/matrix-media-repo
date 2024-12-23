package r0

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/api/_routers"
	"github.com/t2bot/matrix-media-repo/datastores"
	"github.com/t2bot/matrix-media-repo/pipelines/pipeline_download"
	"github.com/t2bot/matrix-media-repo/util"

	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

func DownloadMediaUser(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	return DownloadMedia(r, rctx, _apimeta.AuthContext{User: user})
}

func DownloadMedia(r *http.Request, rctx rcontext.RequestContext, auth _apimeta.AuthContext) interface{} {
	server := _routers.GetParam("server", r)
	mediaId := _routers.GetParam("mediaId", r)
	filename := _routers.GetParam("filename", r)
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

	recordOnly := false
	if r.Method == http.MethodHead {
		rctx.Log.Debug("HEAD request received - changing parameters")
		//downloadRemote = false // we allow the download to go through to ensure proper metadata is returned
		recordOnly = true
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"mediaId":        mediaId,
		"server":         server,
		"filename":       filename,
		"allowRemote":    downloadRemote,
		"allowRedirect":  canRedirect,
		"authUserId":     auth.User.UserId,
		"authServerName": auth.Server.ServerName,
	})

	if util.IsHostIgnored(server) {
		if auth.User.UserId == "" || !util.IsGlobalAdmin(auth.User.UserId) {
			rctx.Log.Warn("Request blocked due to domain being ignored.")
			return _responses.MediaBlocked()
		}
	}

	media, stream, err := pipeline_download.Execute(rctx, server, mediaId, pipeline_download.DownloadOpts{
		FetchRemoteIfNeeded: downloadRemote,
		BlockForReadUntil:   blockFor,
		CanRedirect:         canRedirect,
		RecordOnly:          recordOnly,
		AuthProvided:        auth.IsAuthenticated(),
	})
	if err != nil {
		var redirect datastores.RedirectError
		if errors.Is(err, common.ErrMediaNotFound) {
			return _responses.NotFoundError()
		} else if errors.Is(err, common.ErrRestrictedAuth) {
			return _responses.ErrorResponse{
				Code:         common.ErrCodeNotFound,
				Message:      "authentication is required to download this media",
				InternalCode: common.ErrCodeNotFound, // used to determine http status code
			}
		} else if errors.Is(err, common.ErrMediaTooLarge) {
			return _responses.RequestTooLarge()
		} else if errors.Is(err, common.ErrRateLimitExceeded) {
			return _responses.RateLimitReached()
		} else if errors.Is(err, common.ErrMediaQuarantined) {
			rctx.Log.Debug("Quarantined media accessed. Has stream? ", stream != nil)
			if stream != nil {
				return _responses.MakeQuarantinedImageResponse(stream)
			} else {
				return _responses.NotFoundError() // We lie for security
			}
		} else if errors.Is(err, common.ErrMediaNotYetUploaded) {
			return _responses.NotYetUploaded()
		} else if errors.As(err, &redirect) {
			return _responses.Redirect(redirect.RedirectUrl)
		}
		rctx.Log.Error("Unexpected error locating media: ", err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected Error")
	}

	if filename == "" {
		filename = media.UploadName
	}

	return &_responses.DownloadResponse{
		ContentType:       media.ContentType,
		Filename:          filename,
		SizeBytes:         media.SizeBytes,
		Data:              stream,
		TargetDisposition: "infer",
	}
}
