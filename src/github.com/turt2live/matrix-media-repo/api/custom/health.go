package custom

import (
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
)

type HealthzResponse struct {
	OK     bool   `json:"ok"`
	Status string `json:"status"`
}

func GetHealthz(r *http.Request, log *logrus.Entry, user api.UserInfo) interface{} {
	return &HealthzResponse{
		OK:     true,
		Status: "Probably not dead",
	}
}
