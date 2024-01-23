package v1

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/pipelines/pipeline_create"
	"github.com/t2bot/matrix-media-repo/util"
)

type MediaCreatedResponse struct {
	ContentUri string `json:"content_uri"`
	ExpiresTs  int64  `json:"unused_expires_at"`
}

func CreateMedia(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	id, err := pipeline_create.Execute(rctx, r.Host, user.UserId, pipeline_create.DefaultExpirationTime)
	if err != nil {
		rctx.Log.Error("Unexpected error creating media ID:", err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("unexpected error")
	}

	return &MediaCreatedResponse{
		ContentUri: util.MxcUri(id.Origin, id.MediaId),
		ExpiresTs:  id.ExpiresTs,
	}
}
