package custom

import (
	"net/http"

	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/common/version"
)

func GetVersion(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	unstableFeatures := make(map[string]bool)

	return &_responses.DoNotCacheResponse{
		Payload: map[string]interface{}{
			"Version":           version.Version,
			"GitCommit":         version.GitCommit,
			"unstable_features": unstableFeatures,
		},
	}
}
