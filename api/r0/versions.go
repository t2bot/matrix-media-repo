package r0

import (
	"net/http"
	"slices"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/matrix"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

func ClientVersions(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	versions, err := matrix.ClientVersions(rctx, r.Host, user.UserId, user.AccessToken, r.RemoteAddr)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("unable to get versions")
	}

	// This is where we'd add our feature/version support as needed
	if versions.Versions == nil {
		versions.Versions = make([]string, 1)
	}

	// We add v1.11 by force, even though we can't reliably say the rest of the server implements it. This
	// is because server admins which point `/versions` at us are effectively opting in to whatever features
	// we need to advertise support for. In our case, it's at least Authenticated Media (MSC3916).
	if !slices.Contains(versions.Versions, "v1.11") {
		versions.Versions = append(versions.Versions, "v1.11")
	}

	return versions
}
