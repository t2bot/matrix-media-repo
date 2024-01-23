package custom

import (
	"net/http"

	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

type HealthzResponse struct {
	OK     bool   `json:"ok"`
	Status string `json:"status"`
}

func GetHealthz(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	return &_responses.DoNotCacheResponse{
		Payload: &HealthzResponse{
			OK:     true,
			Status: "Probably not dead",
		},
	}
}
