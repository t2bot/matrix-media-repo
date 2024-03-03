package custom

import (
	"net/http"

	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/api/apimeta"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/common/version"
)

func GetVersion(r *http.Request, rctx rcontext.RequestContext, user apimeta.UserInfo) interface{} {
	unstableFeatures := make(map[string]bool)

	return &_responses.DoNotCacheResponse{
		Payload: map[string]interface{}{
			"Version":           version.Version,
			"GitCommit":         version.GitCommit,
			"unstable_features": unstableFeatures,
		},
	}
}
