package custom

import (
	"net/http"

	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

type HealthzResponse struct {
	OK     bool   `json:"ok"`
	Status string `json:"status"`
}

func GetHealthz(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	return &api.DoNotCacheResponse{
		Payload: &HealthzResponse{
			OK:     true,
			Status: "Probably not dead",
		},
	}
}
