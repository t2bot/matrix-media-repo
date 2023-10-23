package unstable

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/api/_routers"
	"github.com/turt2live/matrix-media-repo/api/r0"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/util"
)

func ClientUploadMediaSync(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	// We're a bit fancy here. Instead of mirroring the "upload sync" endpoint to include restricted media, we
	// internally create an async media ID then claim it immediately.

	id, err := restrictAsyncMediaId(rctx, r.Host, user.UserId)
	if err != nil {
		rctx.Log.Error("Unexpected error creating media ID:", err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("unexpected error")
	}

	r = _routers.ForceSetParam("server", id.Origin, r)
	r = _routers.ForceSetParam("mediaId", id.MediaId, r)

	resp := r0.UploadMediaAsync(r, rctx, user)
	if _, ok := resp.(*r0.MediaUploadedResponse); ok {
		return &r0.MediaUploadedResponse{
			ContentUri: util.MxcUri(id.Origin, id.MediaId),
		}
	}
	return resp
}
