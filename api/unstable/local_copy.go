package unstable

import (
	"net/http"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/api/_routers"
	"github.com/turt2live/matrix-media-repo/api/r0"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/datastores"
	"github.com/turt2live/matrix-media-repo/pipelines/pipeline_download"
	"github.com/turt2live/matrix-media-repo/pipelines/pipeline_upload"
	"github.com/turt2live/matrix-media-repo/util"
)

func LocalCopy(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	server := _routers.GetParam("server", r)
	mediaId := _routers.GetParam("mediaId", r)
	allowRemote := r.URL.Query().Get("allow_remote")

	rctx.Log.Warn("This endpoint is deprecated. See https://github.com/turt2live/matrix-media-repo/issues/422")

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

	if r.Host == server {
		return _responses.BadRequest("Attempted to clone media to the same origin")
	}

	// TODO: There's a lot of room for improvement here. Instead of re-uploading media, we should just update the DB.

	record, stream, err := pipeline_download.Execute(rctx, server, mediaId, pipeline_download.DownloadOpts{
		FetchRemoteIfNeeded: downloadRemote,
		StartByte:           -1,
		EndByte:             -1,
		BlockForReadUntil:   30 * time.Second,
		RecordOnly:          false,
	})
	// Error handling copied from download endpoint
	if err != nil {
		if err == common.ErrMediaNotFound {
			return _responses.NotFoundError()
		} else if err == common.ErrMediaTooLarge {
			return _responses.RequestTooLarge()
		} else if err == common.ErrMediaQuarantined {
			rctx.Log.Debug("Quarantined media accessed. Has stream? ", stream != nil)
			if stream != nil {
				return _responses.MakeQuarantinedImageResponse(stream)
			} else {
				return _responses.NotFoundError() // We lie for security
			}
		} else if err == common.ErrMediaNotYetUploaded {
			return _responses.NotYetUploaded()
		}
		rctx.Log.Error("Unexpected error locating media: ", err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected Error")
	}

	record, err = pipeline_upload.Execute(rctx, server, mediaId, stream, record.ContentType, record.UploadName, user.UserId, datastores.LocalMediaKind)
	// Error handling copied from upload(sync) endpoint
	if err != nil {
		if err == common.ErrQuotaExceeded {
			return _responses.QuotaExceeded()
		}
		rctx.Log.Error("Unexpected error uploading media: ", err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected Error")
	}

	blurhash, err := database.GetInstance().Blurhashes.Prepare(rctx).Get(record.Sha256Hash)
	if err != nil {
		rctx.Log.Warn("Unexpected error getting media's blurhash from DB: ", err)
		sentry.CaptureException(err)
	}

	return &r0.MediaUploadedResponse{
		ContentUri: util.MxcUri(record.Origin, record.MediaId),
		Blurhash:   blurhash,
	}
}
