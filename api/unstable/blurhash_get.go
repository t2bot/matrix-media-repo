package unstable

import (
	"github.com/getsentry/sentry-go"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/download_controller"
	"github.com/turt2live/matrix-media-repo/controllers/info_controller"
)

func GetBlurhash(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	params := mux.Vars(r)

	server := params["server"]
	mediaId := params["mediaId"]

	m, err := download_controller.FindMediaRecord(server, mediaId, true, rctx)
	if err != nil {
		if err == common.ErrMediaNotFound {
			return api.NotFoundError()
		} else if err == common.ErrMediaTooLarge {
			return api.RequestTooLarge()
		} else if err == common.ErrMediaQuarantined {
			return api.NotFoundError() // We lie for security
		}
		rctx.Log.Error("Unexpected error locating media: " + err.Error())
		sentry.CaptureException(err)
		return api.InternalServerError("Unexpected Error")
	}

	hash, err := info_controller.GetOrCalculateBlurhash(m, rctx)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return api.InternalServerError("Unexpected Error")
	}

	return &map[string]string{"blurhash": hash}
}
