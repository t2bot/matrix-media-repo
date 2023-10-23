package unstable

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	v1 "github.com/turt2live/matrix-media-repo/api/v1"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/pipelines/pipeline_create"
	"github.com/turt2live/matrix-media-repo/util"
)

func ClientCreateMedia(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	id, err := restrictAsyncMediaId(rctx, r.Host, user.UserId)
	if err != nil {
		rctx.Log.Error("Unexpected error creating media ID:", err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("unexpected error")
	}

	return &v1.MediaCreatedResponse{
		ContentUri: util.MxcUri(id.Origin, id.MediaId),
		ExpiresTs:  id.ExpiresTs,
	}
}

func restrictAsyncMediaId(ctx rcontext.RequestContext, host string, userId string) (*database.DbExpiringMedia, error) {
	id, err := pipeline_create.Execute(ctx, host, userId, pipeline_create.DefaultExpirationTime)
	if err != nil {
		return nil, err
	}

	db := database.GetInstance().RestrictedMedia.Prepare(ctx)
	err = db.Insert(id.Origin, id.MediaId, database.RestrictedToUser, id.UserId)
	if err != nil {
		// Try to clean up the expiring record, but don't fail if it fails
		err2 := database.GetInstance().ExpiringMedia.Prepare(ctx).SetExpiry(id.Origin, id.MediaId, util.NowMillis())
		if err2 != nil {
			ctx.Log.Warn("Non-fatal error when trying to clean up interstitial expiring media: ", err2)
			sentry.CaptureException(err2)
		}

		return nil, err
	}

	return id, nil
}
