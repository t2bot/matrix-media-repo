package v1

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/pipline/create_pipeline"
	"github.com/turt2live/matrix-media-repo/util"
)

type MediaCreatedResponse struct {
	ContentUri string `json:"content_uri"`
	ExpiresTs  int64  `json:"unused_expires_at"`
}

func CreateMedia(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	id, err := create_pipeline.Execute(rctx, r.Host, user.UserId, create_pipeline.DefaultExpirationTime)
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
