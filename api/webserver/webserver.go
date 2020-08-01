package webserver

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/didip/tollbooth"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/api/custom"
	"github.com/turt2live/matrix-media-repo/api/features"
	"github.com/turt2live/matrix-media-repo/api/r0"
	"github.com/turt2live/matrix-media-repo/api/unstable"
	"github.com/turt2live/matrix-media-repo/api/webserver/debug"
	"github.com/turt2live/matrix-media-repo/common/config"
)

type route struct {
	method  string
	handler handler
}

var srv *http.Server
var waitGroup = &sync.WaitGroup{}
var reload = false

func Init() *sync.WaitGroup {
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
	purgeQuarantinedHandler := handler{api.AccessTokenRequiredRoute(custom.PurgeQuarantined), "purge_quarantined", counter, false}
	purgeUserMediaHandler := handler{api.AccessTokenRequiredRoute(custom.PurgeUserMedia), "purge_user_media", counter, false}
	purgeRoomHandler := handler{api.AccessTokenRequiredRoute(custom.PurgeRoomMedia), "purge_room_media", counter, false}
	purgeDomainHandler := handler{api.AccessTokenRequiredRoute(custom.PurgeDomainMedia), "purge_domain_media", counter, false}
	purgeOldHandler := handler{api.RepoAdminRoute(custom.PurgeOldMedia), "purge_old_media", counter, false}
	quarantineHandler := handler{api.AccessTokenRequiredRoute(custom.QuarantineMedia), "quarantine_media", counter, false}
	quarantineRoomHandler := handler{api.AccessTokenRequiredRoute(custom.QuarantineRoomMedia), "quarantine_room", counter, false}
	quarantineUserHandler := handler{api.AccessTokenRequiredRoute(custom.QuarantineUserMedia), "quarantine_user", counter, false}
	quarantineDomainHandler := handler{api.AccessTokenRequiredRoute(custom.QuarantineDomainMedia), "quarantine_domain", counter, false}
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
	exportUserDataHandler := handler{api.AccessTokenRequiredRoute(custom.ExportUserData), "export_user_data", counter, false}
	exportServerDataHandler := handler{api.AccessTokenRequiredRoute(custom.ExportServerData), "export_server_data", counter, false}
	viewExportHandler := handler{api.AccessTokenOptionalRoute(custom.ViewExport), "view_export", counter, false}
	getExportMetadataHandler := handler{api.AccessTokenOptionalRoute(custom.GetExportMetadata), "get_export_metadata", counter, false}
	downloadExportPartHandler := handler{api.AccessTokenOptionalRoute(custom.DownloadExportPart), "download_export_part", counter, false}
	deleteExportHandler := handler{api.AccessTokenOptionalRoute(custom.DeleteExport), "delete_export", counter, false}
	startImportHandler := handler{api.RepoAdminRoute(custom.StartImport), "start_import", counter, false}
	appendToImportHandler := handler{api.RepoAdminRoute(custom.AppendToImport), "append_to_import", counter, false}
	stopImportHandler := handler{api.RepoAdminRoute(custom.StopImport), "stop_import", counter, false}
	versionHandler := handler{api.AccessTokenOptionalRoute(custom.GetVersion), "get_version", counter, false}
	blurhashRenderHandler := handler{api.AccessTokenRequiredRoute(unstable.RenderBlurhash), "render_blurhash", counter, false}
	blurhashCalcHandler := handler{api.AccessTokenRequiredRoute(unstable.GetBlurhash), "calculate_blurhash", counter, false}
	ipfsDownloadHandler := handler{api.AccessTokenOptionalRoute(unstable.IPFSDownload), "ipfs_download", counter, false}
	logoutHandler := handler{api.AccessTokenRequiredRoute(r0.Logout), "logout", counter, false}
	logoutAllHandler := handler{api.AccessTokenRequiredRoute(r0.LogoutAll), "logout_all", counter, false}
	getMediaAttrsHandler := handler{api.AccessTokenRequiredRoute(custom.GetAttributes), "get_media_attributes", counter, false}
	setMediaAttrsHandler := handler{api.AccessTokenRequiredRoute(custom.SetAttributes), "set_media_attributes", counter, false}

	routes := make(map[string]route)
	// r0 is typically clients and v1 is typically servers. v1 is deprecated.
	// unstable is, well, unstable. unstable/io.t2bot.media is to comply with MSC2324
	versions := []string{"r0", "v1", "unstable", "unstable/io.t2bot.media"}

	// Things that don't need a version
	routes["/_matrix/media/version"] = route{"GET", versionHandler}

	for _, version := range versions {
		// Standard routes we have to handle
		routes["/_matrix/media/"+version+"/upload"] = route{"POST", uploadHandler}
		routes["/_matrix/media/"+version+"/download/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[^/]+}"] = route{"GET", downloadHandler}
		routes["/_matrix/media/"+version+"/download/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[^/]+}/{filename:.+}"] = route{"GET", downloadHandler}
		routes["/_matrix/media/"+version+"/thumbnail/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[^/]+}"] = route{"GET", thumbnailHandler}
		routes["/_matrix/media/"+version+"/preview_url"] = route{"GET", previewUrlHandler}
		routes["/_matrix/media/"+version+"/identicon/{seed:.*}"] = route{"GET", identiconHandler}
		routes["/_matrix/media/"+version+"/config"] = route{"GET", configHandler}
		routes["/_matrix/client/"+version+"/logout"] = route{"POST", logoutHandler}
		routes["/_matrix/client/"+version+"/logout/all"] = route{"POST", logoutAllHandler}

		// Routes that we define but are not part of the spec (management)
		routes["/_matrix/media/"+version+"/admin/purge_remote"] = route{"POST", purgeRemote} // deprecated
		routes["/_matrix/media/"+version+"/admin/purge/remote"] = route{"POST", purgeRemote}
		routes["/_matrix/media/"+version+"/admin/purge/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[^/]+}"] = route{"POST", purgeOneHandler}
		routes["/_matrix/media/"+version+"/admin/purge/quarantined"] = route{"POST", purgeQuarantinedHandler}
		routes["/_matrix/media/"+version+"/admin/purge/user/{userId:[^/]+}"] = route{"POST", purgeUserMediaHandler}
		routes["/_matrix/media/"+version+"/admin/purge/room/{roomId:[^/]+}"] = route{"POST", purgeRoomHandler}
		routes["/_matrix/media/"+version+"/admin/purge/server/{serverName:[^/]+}"] = route{"POST", purgeDomainHandler}
		routes["/_matrix/media/"+version+"/admin/purge/old"] = route{"POST", purgeOldHandler}
		routes["/_matrix/media/"+version+"/admin/room/{roomId:[^/]+}/quarantine"] = route{"POST", quarantineRoomHandler} // deprecated
		routes["/_matrix/media/"+version+"/admin/quarantine/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[^/]+}"] = route{"POST", quarantineHandler}
		routes["/_matrix/media/"+version+"/admin/quarantine/room/{roomId:[^/]+}"] = route{"POST", quarantineRoomHandler}
		routes["/_matrix/media/"+version+"/admin/quarantine/user/{userId:[^/]+}"] = route{"POST", quarantineUserHandler}
		routes["/_matrix/media/"+version+"/admin/quarantine/server/{serverName:[^/]+}"] = route{"POST", quarantineDomainHandler}
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
		routes["/_matrix/media/"+version+"/admin/user/{userId:[^/]+}/export"] = route{"POST", exportUserDataHandler}
		routes["/_matrix/media/"+version+"/admin/server/{serverName:[^/]+}/export"] = route{"POST", exportServerDataHandler}
		routes["/_matrix/media/"+version+"/admin/export/{exportId:[a-zA-Z0-9.:\\-_]+}/view"] = route{"GET", viewExportHandler}
		routes["/_matrix/media/"+version+"/admin/export/{exportId:[a-zA-Z0-9.:\\-_]+}/metadata"] = route{"GET", getExportMetadataHandler}
		routes["/_matrix/media/"+version+"/admin/export/{exportId:[a-zA-Z0-9.:\\-_]+}/part/{partId:[0-9]+}"] = route{"GET", downloadExportPartHandler}
		routes["/_matrix/media/"+version+"/admin/export/{exportId:[a-zA-Z0-9.:\\-_]+}/delete"] = route{"DELETE", deleteExportHandler}
		routes["/_matrix/media/"+version+"/admin/import"] = route{"POST", startImportHandler}
		routes["/_matrix/media/"+version+"/admin/import/{importId:[a-zA-Z0-9.:\\-_]+}/part"] = route{"POST", appendToImportHandler}
		routes["/_matrix/media/"+version+"/admin/import/{importId:[a-zA-Z0-9.:\\-_]+}/close"] = route{"POST", stopImportHandler}
		routes["/_matrix/media/"+version+"/admin/media/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[^/]+}/attributes"] = route{"GET", getMediaAttrsHandler}
		routes["/_matrix/media/"+version+"/admin/media/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[^/]+}/attributes/set"] = route{"POST", setMediaAttrsHandler}

		// Routes that we should handle but aren't in the media namespace (synapse compat)
		routes["/_matrix/client/"+version+"/admin/purge_media_cache"] = route{"POST", purgeRemote}
		routes["/_matrix/client/"+version+"/admin/quarantine_media/{roomId:[^/]+}"] = route{"POST", quarantineRoomHandler}

		if strings.Index(version, "unstable") == 0 {
			routes["/_matrix/media/"+version+"/local_copy/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[^/]+}"] = route{"GET", localCopyHandler}
			routes["/_matrix/media/"+version+"/info/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[^/]+}"] = route{"GET", infoHandler}
			routes["/_matrix/media/"+version+"/download/{server:[a-zA-Z0-9.:\\-_]+}/{mediaId:[^/]+}"] = route{"DELETE", purgeOneHandler}
		}
	}

	if config.Get().Features.MSC2448Blurhash.Enabled {
		routes[features.MSC2448UploadRoute] = route{"POST", uploadHandler}
		routes[features.MSC2448GetHashRoute] = route{"GET", blurhashCalcHandler}
		routes[features.MSC2448AltRenderRoute] = route{"GET", blurhashRenderHandler}
	}

	if config.Get().Features.IPFS.Enabled {
		routes[features.IPFSDownloadRoute] = route{"GET", ipfsDownloadHandler}
		routes[features.IPFSLiveDownloadRouteR0] = route{"GET", ipfsDownloadHandler}
		routes[features.IPFSLiveDownloadRouteV1] = route{"GET", ipfsDownloadHandler}
		routes[features.IPFSLiveDownloadRouteUnstable] = route{"GET", ipfsDownloadHandler}
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

	address := net.JoinHostPort(config.Get().General.BindAddress, strconv.Itoa(config.Get().General.Port))
	httpMux := http.NewServeMux()
	httpMux.Handle("/", handler)

	pprofSecret := os.Getenv("MEDIA_PPROF_SECRET_KEY")
	if pprofSecret != "" {
		logrus.Warn("Enabling pprof endpoints")
		debug.BindPprofEndpoints(httpMux, pprofSecret)
	}

	srv = &http.Server{Addr: address, Handler: httpMux}
	reload = false

	go func() {
		logrus.WithField("address", address).Info("Started up. Listening at http://" + address)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			logrus.Fatal(err)
		}

		// Only notify the main thread that we're done if we're actually done
		srv = nil
		if !reload {
			waitGroup.Done()
		}
	}()

	return waitGroup
}

func Reload() {
	reload = true

	// Stop the server first
	Stop()

	// Reload the web server, ignoring the wait group (because we don't care to wait here)
	Init()
}

func Stop() {
	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			panic(err)
		}
	}
}
