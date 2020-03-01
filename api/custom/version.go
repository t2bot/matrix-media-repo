package custom

import (
	"net/http"

	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/common/version"
)

func GetVersion(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	unstableFeatures := make(map[string]bool)
	unstableFeatures["xyz.amorgan.blurhash"] = rctx.Config.Features.MSC2448Blurhash.Enabled

	return &api.DoNotCacheResponse{
		Payload: map[string]interface{}{
			"Version":           version.Version,
			"GitCommit":         version.GitCommit,
			"unstable_features": unstableFeatures,
		},
	}
}
