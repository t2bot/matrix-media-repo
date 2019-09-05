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

	optionsHandler := handler{api.EmptyResponseHandler, "options_request", counter, false}
	uploadHandler := handler{api.AccessTokenRequiredRoute(r0.UploadMedia), "upload", counter, false}
	downloadHandler := handler{api.AccessTokenOptionalRoute(r0.DownloadMedia), "download", counter, false}
	thumbnailHandler := handler{api.AccessTokenOptionalRoute(r0.ThumbnailMedia), "thumbnail", counter, false}
	previewUrlHandler := handler{api.AccessTokenRequiredRoute(r0.PreviewUrl), "url_preview", counter, false}
	identiconHandler := handler{api.AccessTokenOptionalRoute(r0.Identicon), "identicon", counter, false}
	purgeRemote := handler{api.RepoAdminRoute(custom.PurgeRemoteMedia), "purge_remote_media", counter, false}
	purgeOneHandler := handler{api.AccessTokenRequiredRoute(custom.PurgeIndividualRecord), "purge_individual_media", counter, false}
	purgeQuarantinedHandler := handler{api.AccessTokenRequiredRoute(custom.PurgeQurantined), "purge_quarantined", counter, false}
	quarantineHandler := handler{api.AccessTokenRequiredRoute(custom.QuarantineMedia), "quarantine_media", counter, false}
	quarantineRoomHandler := handler{api.AccessTokenRequiredRoute(custom.QuarantineRoomMedia), "quarantine_room", counter, false}
	localCopyHandler := handler{api.AccessTokenRequiredRoute(unstable.LocalCopy), "local_copy", counter, false}
	infoHandler := handler{api.AccessTokenRequiredRoute(unstable.MediaInfo), "info", counter, false}
	configHandler := handler{api.AccessTokenRequiredRoute(r0.PublicConfig), "config", counter, false}
	storageEstimateHandler := handler{api.RepoAdminRoute(custom.GetDatastoreStorageEstimate), "get_storage_estimate", counter, false}
	datastoreListHandler := handler{api.RepoAdminRoute(custom.GetDatastores), "list_datastores", counter, false}
	dsTransferHandler := handler{api.RepoAdminRoute(custom.MigrateBetweenDatastores), "datastore_transfer", counter, false}
	fedTestHandler := handler{api.RepoAdminRoute(custom.GetFederationInfo), "federation_test", counter, false}
	healthzHandler := handler{api.AccessTokenOptionalRoute(custom.GetHealthz), "healthz", counter, true}
	domainUsageHandler := handler{api.RepoAdminRoute(custom.GetDomainUsage), "domain_usage", counter, false}
	userUsageHandler := handler{api.RepoAdminRoute(custom.GetUserUsage), "user_usage", counter, false}
	uploadsUsageHandler := handler{api.RepoAdminRoute(custom.GetUploadsUsage), "uploads_usage", counter, false}
	getBackgroundTaskHandler := handler{api.RepoAdminRoute(custom.GetTask), "get_background_task", counter, false}
	listAllBackgroundTasksHandler := handler{api.RepoAdminRoute(custom.ListAllTasks), "list_all_background_tasks", counter, false}
	listUnfinishedBackgroundTasksHandler := handler{api.RepoAdminRoute(custom.ListUnfinishedTasks), "list_unfinished_background_tasks", counter, false}

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
		routes["/_matrix/media/"+version+"/admin/purge_remote"] = route{"POST", purgeRemote}
		routes["/_matrix/media/"+version+"/admin/purge/remote"] = route{"POST", purgeRemote}
		routes["/_matrix/media/"+version+"/admin/purge/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[a-zA-Z0-9.\\-_]+}"] = route{"POST", purgeOneHandler}
		routes["/_matrix/media/"+version+"/admin/purge/quarantined"] = route{"POST", purgeQuarantinedHandler}
		routes["/_matrix/media/"+version+"/admin/quarantine/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[a-zA-Z0-9.\\-_]+}"] = route{"POST", quarantineHandler}
		routes["/_matrix/media/"+version+"/admin/room/{roomId:[^/]+}/quarantine"] = route{"POST", quarantineRoomHandler}
		routes["/_matrix/media/"+version+"/admin/datastores/{datastoreId:[^/]+}/size_estimate"] = route{"GET", storageEstimateHandler}
		routes["/_matrix/media/"+version+"/admin/datastores"] = route{"GET", datastoreListHandler}
		routes["/_matrix/media/"+version+"/admin/datastores/{sourceDsId:[^/]+}/transfer_to/{targetDsId:[^/]+}"] = route{"POST", dsTransferHandler}
		routes["/_matrix/media/"+version+"/admin/federation/test/{serverName:[a-zA-Z0-9.:\\-_]+}"] = route{"GET", fedTestHandler}
		routes["/_matrix/media/"+version+"/admin/usage/{serverName:[a-zA-Z0-9.:\\-_]+}"] = route{"GET", domainUsageHandler}
		routes["/_matrix/media/"+version+"/admin/usage/{serverName:[a-zA-Z0-9.:\\-_]+}/users"] = route{"GET", userUsageHandler}
		routes["/_matrix/media/"+version+"/admin/usage/{serverName:[a-zA-Z0-9.:\\-_]+}/uploads"] = route{"GET", uploadsUsageHandler}
		routes["/_matrix/media/"+version+"/admin/tasks/{taskId:[0-9]+}"] = route{"GET", getBackgroundTaskHandler}
		routes["/_matrix/media/"+version+"/admin/tasks/all"] = route{"GET", listAllBackgroundTasksHandler}
		routes["/_matrix/media/"+version+"/admin/tasks/unfinished"] = route{"GET", listUnfinishedBackgroundTasksHandler}

		// Routes that we should handle but aren't in the media namespace (synapse compat)
		routes["/_matrix/client/"+version+"/admin/purge_media_cache"] = route{"POST", purgeRemote}
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

	// Health check endpoints
	rtr.Handle("/healthz", healthzHandler).Methods("OPTIONS", "GET")

	rtr.NotFoundHandler = handler{api.NotFoundHandler, "not_found", counter, true}
	rtr.MethodNotAllowedHandler = handler{api.MethodNotAllowedHandler, "method_not_allowed", counter, true}

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

	// pprof endpoints
	//httpMux.HandleFunc("/debug/pprof", pprof.Index)
	//httpMux.HandleFunc("/debug/pprof/heap", pprof.Index)
	//httpMux.HandleFunc("/debug/pprof/allocs", pprof.Index)
	//httpMux.HandleFunc("/debug/pprof/block", pprof.Index)
	//httpMux.HandleFunc("/debug/pprof/profile", pprof.Index)
	//httpMux.HandleFunc("/debug/pprof/trace", pprof.Index)

	logrus.WithField("address", address).Info("Started up. Listening at http://" + address)
	logrus.Fatal(http.ListenAndServe(address, httpMux))
}
