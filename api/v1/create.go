package v1

import (
	"errors"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/api/apimeta"
	"github.com/t2bot/matrix-media-repo/api/responses"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/pipelines/pipeline_create"
	"github.com/t2bot/matrix-media-repo/util"
)

type MediaCreatedResponse struct {
	ContentUri string `json:"content_uri"`
	ExpiresTs  int64  `json:"unused_expires_at"`
}

func CreateMedia(r *http.Request, rctx rcontext.RequestContext, user apimeta.UserInfo) interface{} {
	id, err := pipeline_create.Execute(rctx, r.Host, user.UserId, pipeline_create.DefaultExpirationTime)
	if err != nil {
		rctx.Log.Error("Unexpected error creating media ID:", err)
		sentry.CaptureException(err)
		return responses.InternalServerError(errors.New("unexpected error"))
	}

	return &MediaCreatedResponse{
		ContentUri: util.MxcUri(id.Origin, id.MediaId),
		ExpiresTs:  id.ExpiresTs,
	}
}
