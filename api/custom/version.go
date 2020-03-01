package custom

import (
	"net/http"

	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/common/version"
)

func GetVersion(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	return &api.DoNotCacheResponse{
		Payload: map[string]interface{}{
			"Version":   version.Version,
			"GitCommit": version.GitCommit,
			"unstable_features": []string{
				"xyz.amorgan.blurhash",
			},
		},
	}
}
