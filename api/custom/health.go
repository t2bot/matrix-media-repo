package custom

import (
	"net/http"

	"github.com/t2bot/matrix-media-repo/api/apimeta"
	"github.com/t2bot/matrix-media-repo/api/responses"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

type HealthzResponse struct {
	OK     bool   `json:"ok"`
	Status string `json:"status"`
}

func GetHealthz(r *http.Request, rctx rcontext.RequestContext, user apimeta.UserInfo) interface{} {
	return &responses.DoNotCacheResponse{
		Payload: &HealthzResponse{
			OK:     true,
			Status: "Probably not dead",
		},
	}
}
