package r0

import (
	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/matrix"

	"net/http"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

func ClientVersions(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	versions, err := matrix.ClientVersions(rctx, r.Host, user.UserId, user.AccessToken, r.RemoteAddr)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("unable to get versions")
	}
	if versions.UnstableFeatures == nil {
		versions.UnstableFeatures = make(map[string]bool)
	}
	versions.UnstableFeatures["org.matrix.msc3916"] = true
	return versions
}
