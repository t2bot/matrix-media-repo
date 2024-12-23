package unstable

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/api/_routers"
	"github.com/t2bot/matrix-media-repo/api/r0"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/datastores"
	"github.com/t2bot/matrix-media-repo/pipelines/pipeline_download"
	"github.com/t2bot/matrix-media-repo/pipelines/pipeline_upload"
	"github.com/t2bot/matrix-media-repo/util"
)

func LocalCopy(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	server := _routers.GetParam("server", r)
	mediaId := _routers.GetParam("mediaId", r)
	allowRemote := r.URL.Query().Get("allow_remote")

	rctx.Log.Warn("This endpoint is deprecated. See https://github.com/t2bot/matrix-media-repo/issues/422")

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

	if util.IsHostIgnored(server) && !util.IsGlobalAdmin(user.UserId) {
		rctx.Log.Warn("Request blocked due to domain being ignored.")
		return _responses.MediaBlocked()
	}

	if r.Host == server {
		return _responses.BadRequest("Attempted to clone media to the same origin")
	}

	// TODO: There's a lot of room for improvement here. Instead of re-uploading media, we should just update the DB.

	record, stream, err := pipeline_download.Execute(rctx, server, mediaId, pipeline_download.DownloadOpts{
		FetchRemoteIfNeeded: downloadRemote,
		BlockForReadUntil:   30 * time.Second,
		RecordOnly:          false,
	})
	// Error handling copied from download endpoint
	if err != nil {
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
		}
		rctx.Log.Error("Unexpected error locating media: ", err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected Error")
	}

	record, err = pipeline_upload.Execute(rctx, server, mediaId, stream, record.ContentType, record.UploadName, user.UserId, datastores.LocalMediaKind)
	// Error handling copied from upload(sync) endpoint
	if err != nil {
		if errors.Is(err, common.ErrQuotaExceeded) {
			return _responses.QuotaExceeded()
		}
		rctx.Log.Error("Unexpected error uploading media: ", err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected Error")
	}

	return &r0.MediaUploadedResponse{
		ContentUri: util.MxcUri(record.Origin, record.MediaId),
	}
}
