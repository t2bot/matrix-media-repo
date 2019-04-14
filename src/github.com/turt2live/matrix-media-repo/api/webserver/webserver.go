package webserver

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/didip/tollbooth"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/api/custom"
	"github.com/turt2live/matrix-media-repo/api/r0"
	"github.com/turt2live/matrix-media-repo/api/unstable"
	"github.com/turt2live/matrix-media-repo/common/config"
)

type route struct {
	method  string
	handler handler
}

func Init() {
	rtr := mux.NewRouter()
	counter := &requestCounter{}

	optionsHandler := handler{api.EmptyResponseHandler, "options_request", counter}
	uploadHandler := handler{api.AccessTokenRequiredRoute(r0.UploadMedia), "upload", counter}
	downloadHandler := handler{api.AccessTokenOptionalRoute(r0.DownloadMedia), "download", counter}
	thumbnailHandler := handler{api.AccessTokenOptionalRoute(r0.ThumbnailMedia), "thumbnail", counter}
	previewUrlHandler := handler{api.AccessTokenRequiredRoute(r0.PreviewUrl), "url_preview", counter}
	identiconHandler := handler{api.AccessTokenOptionalRoute(r0.Identicon), "identicon", counter}
	purgeHandler := handler{api.RepoAdminRoute(custom.PurgeRemoteMedia), "purge_remote_media", counter}
	quarantineHandler := handler{api.AccessTokenRequiredRoute(custom.QuarantineMedia), "quarantine_media", counter}
	quarantineRoomHandler := handler{api.AccessTokenRequiredRoute(custom.QuarantineRoomMedia), "quarantine_room", counter}
	localCopyHandler := handler{api.AccessTokenRequiredRoute(unstable.LocalCopy), "local_copy", counter}
	infoHandler := handler{api.AccessTokenRequiredRoute(unstable.MediaInfo), "info", counter}
	configHandler := handler{api.AccessTokenRequiredRoute(r0.PublicConfig), "config", counter}

	routes := make(map[string]route)
	versions := []string{"r0", "v1", "unstable"} // r0 is typically clients and v1 is typically servers. v1 is deprecated.

	for _, version := range versions {
		// Standard routes we have to handle
		routes["/_matrix/media/"+version+"/upload"] = route{"POST", uploadHandler}
		routes["/_matrix/media/"+version+"/download/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[a-zA-Z0-9.\\-_]+}"] = route{"GET", downloadHandler}
		routes["/_matrix/media/"+version+"/download/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[a-zA-Z0-9.\\-_]+}/{filename:.+}"] = route{"GET", downloadHandler}
		routes["/_matrix/media/"+version+"/thumbnail/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[a-zA-Z0-9.\\-_]+}"] = route{"GET", thumbnailHandler}
		routes["/_matrix/media/"+version+"/preview_url"] = route{"GET", previewUrlHandler}
		routes["/_matrix/media/"+version+"/identicon/{seed:.*}"] = route{"GET", identiconHandler}
		routes["/_matrix/media/"+version+"/config"] = route{"GET", configHandler}

		// Routes that we define but are not part of the spec (management)
		routes["/_matrix/media/"+version+"/admin/purge_remote"] = route{"POST", purgeHandler}
		routes["/_matrix/media/"+version+"/admin/quarantine/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[a-zA-Z0-9.\\-_]+}"] = route{"POST", quarantineHandler}
		routes["/_matrix/media/"+version+"/admin/room/{roomId:[^/]+}/quarantine"] = route{"POST", quarantineRoomHandler}

		// Routes that we should handle but aren't in the media namespace (synapse compat)
		routes["/_matrix/client/"+version+"/admin/purge_media_cache"] = route{"POST", purgeHandler}
		routes["/_matrix/client/"+version+"/admin/quarantine_media/{roomId:[^/]+}"] = route{"POST", quarantineRoomHandler}

		if version == "unstable" {
			routes["/_matrix/media/"+version+"/local_copy/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[a-zA-Z0-9.\\-_]+}"] = route{"GET", localCopyHandler}
			routes["/_matrix/media/"+version+"/info/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[a-zA-Z0-9.\\-_]+}"] = route{"GET", infoHandler}
		}
	}

	for routePath, route := range routes {
		logrus.Info("Registering route: " + route.method + " " + routePath)
		rtr.Handle(routePath, route.handler).Methods(route.method)
		rtr.Handle(routePath, optionsHandler).Methods("OPTIONS")

		// This is a hack to a ensure that trailing slashes also match the routes correctly
		rtr.Handle(routePath+"/", route.handler).Methods(route.method)
		rtr.Handle(routePath+"/", optionsHandler).Methods("OPTIONS")
	}

	rtr.NotFoundHandler = handler{api.NotFoundHandler, "not_found", counter}
	rtr.MethodNotAllowedHandler = handler{api.MethodNotAllowedHandler, "method_not_allowed", counter}

	var handler http.Handler = rtr
	if config.Get().RateLimit.Enabled {
		logrus.Info("Enabling rate limit")
		limiter := tollbooth.NewLimiter(0, nil)
		limiter.SetIPLookups([]string{"X-Forwarded-For", "X-Real-IP", "RemoteAddr"})
		limiter.SetTokenBucketExpirationTTL(time.Hour)
		limiter.SetBurst(config.Get().RateLimit.BurstCount)
		limiter.SetMax(config.Get().RateLimit.RequestsPerSecond)

		b, _ := json.Marshal(api.RateLimitReached())
		limiter.SetMessage(string(b))
		limiter.SetMessageContentType("application/json")

		handler = tollbooth.LimitHandler(limiter, rtr)
	}

	address := config.Get().General.BindAddress + ":" + strconv.Itoa(config.Get().General.Port)
	httpMux := http.NewServeMux()
	httpMux.Handle("/", handler)

	logrus.WithField("address", address).Info("Started up. Listening at http://" + address)
	logrus.Fatal(http.ListenAndServe(address, httpMux))
}
