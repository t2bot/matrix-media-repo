package api

import (
	"fmt"
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api/_debug"
	"github.com/turt2live/matrix-media-repo/api/_routers"
	"github.com/turt2live/matrix-media-repo/api/custom"
	"github.com/turt2live/matrix-media-repo/api/r0"
	"github.com/turt2live/matrix-media-repo/api/unstable"
	v1 "github.com/turt2live/matrix-media-repo/api/v1"
)

const PrefixMedia = "/_matrix/media"
const PrefixClient = "/_matrix/client"

func buildRoutes() http.Handler {
	counter := &_routers.RequestCounter{}
	router := buildPrimaryRouter()

	pprofSecret := os.Getenv("MEDIA_PPROF_SECRET_KEY")
	if pprofSecret != "" {
		logrus.Warn("Enabling pprof/debug http endpoints")
		_debug.BindPprofEndpoints(router, pprofSecret)
	}

	// Standard (spec) features
	register([]string{"POST"}, PrefixMedia, "upload", mxSpecV3Transition, router, makeRoute(_routers.RequireAccessToken(r0.UploadMedia), "upload", counter))
	register([]string{"PUT"}, PrefixMedia, "upload/:server/:mediaId", mxV3, router, makeRoute(_routers.RequireAccessToken(r0.UploadMediaAsync), "upload_async", counter))
	downloadRoute := makeRoute(_routers.OptionalAccessToken(r0.DownloadMedia), "download", counter)
	register([]string{"GET"}, PrefixMedia, "download/:server/:mediaId/:filename", mxSpecV3Transition, router, downloadRoute)
	register([]string{"GET"}, PrefixMedia, "download/:server/:mediaId", mxSpecV3Transition, router, downloadRoute)
	register([]string{"GET"}, PrefixMedia, "thumbnail/:server/:mediaId", mxSpecV3Transition, router, makeRoute(_routers.OptionalAccessToken(r0.ThumbnailMedia), "thumbnail", counter))
	register([]string{"GET"}, PrefixMedia, "preview_url", mxSpecV3TransitionCS, router, makeRoute(_routers.RequireAccessToken(r0.PreviewUrl), "url_preview", counter))
	register([]string{"GET"}, PrefixMedia, "identicon/*seed", mxR0, router, makeRoute(_routers.OptionalAccessToken(r0.Identicon), "identicon", counter))
	register([]string{"GET"}, PrefixMedia, "config", mxSpecV3TransitionCS, router, makeRoute(_routers.RequireAccessToken(r0.PublicConfig), "config", counter))
	register([]string{"POST"}, PrefixClient, "logout", mxSpecV3TransitionCS, router, makeRoute(_routers.RequireAccessToken(r0.Logout), "logout", counter))
	register([]string{"POST"}, PrefixClient, "logout/all", mxSpecV3TransitionCS, router, makeRoute(_routers.RequireAccessToken(r0.LogoutAll), "logout_all", counter))
	register([]string{"POST"}, PrefixMedia, "create", mxV1, router, makeRoute(_routers.RequireAccessToken(v1.CreateMedia), "create", counter))

	// Custom features
	register([]string{"GET"}, PrefixMedia, "local_copy/:server/:mediaId", mxUnstable, router, makeRoute(_routers.RequireAccessToken(unstable.LocalCopy), "local_copy", counter))
	register([]string{"GET"}, PrefixMedia, "info/:server/:mediaId", mxUnstable, router, makeRoute(_routers.RequireAccessToken(unstable.MediaInfo), "info", counter))
	purgeOneRoute := makeRoute(_routers.RequireAccessToken(custom.PurgeIndividualRecord), "purge_individual_media", counter)
	register([]string{"DELETE"}, PrefixMedia, "download/:server/:mediaId", mxUnstable, router, purgeOneRoute)

	// Custom and top-level features
	router.Handler("GET", fmt.Sprintf("%s/version", PrefixMedia), makeRoute(_routers.OptionalAccessToken(custom.GetVersion), "get_version", counter))
	healthzRoute := makeRoute(_routers.OptionalAccessToken(custom.GetHealthz), "healthz", counter) // Note: healthz handling is special in makeRoute()
	router.Handler("GET", "/healthz", healthzRoute)
	router.Handler("HEAD", "/healthz", healthzRoute)

	// All admin routes are unstable only
	purgeRemoteRoute := makeRoute(_routers.RequireRepoAdmin(custom.PurgeRemoteMedia), "purge_remote_media", counter)
	register([]string{"POST"}, PrefixMedia, "admin/purge_remote", mxUnstable, router, purgeRemoteRoute)
	register([]string{"POST"}, PrefixMedia, "admin/purge/remote", mxUnstable, router, purgeRemoteRoute)
	register([]string{"POST"}, PrefixClient, "admin/purge_media_cache", mxUnstable, router, purgeRemoteRoute) // synapse compat
	register([]string{"POST"}, PrefixMedia, "admin/purge/media/:server/:mediaId", mxUnstable, router, purgeOneRoute)
	register([]string{"POST"}, PrefixMedia, "admin/purge/old", mxUnstable, router, makeRoute(_routers.RequireRepoAdmin(custom.PurgeOldMedia), "purge_old_media", counter))
	register([]string{"POST"}, PrefixMedia, "admin/purge/quarantined", mxUnstable, router, makeRoute(_routers.RequireAccessToken(custom.PurgeQuarantined), "purge_quarantined", counter))
	register([]string{"POST"}, PrefixMedia, "admin/purge/user/:userId", mxUnstable, router, makeRoute(_routers.RequireAccessToken(custom.PurgeUserMedia), "purge_user_media", counter))
	register([]string{"POST"}, PrefixMedia, "admin/purge/room/:roomId", mxUnstable, router, makeRoute(_routers.RequireAccessToken(custom.PurgeRoomMedia), "purge_room_media", counter))
	register([]string{"POST"}, PrefixMedia, "admin/purge/server/:serverName", mxUnstable, router, makeRoute(_routers.RequireAccessToken(custom.PurgeDomainMedia), "purge_domain_media", counter))
	quarantineRoomRoute := makeRoute(_routers.RequireAccessToken(custom.QuarantineRoomMedia), "quarantine_room", counter)
	register([]string{"POST"}, PrefixMedia, "admin/quarantine/room/:roomId", mxUnstable, router, quarantineRoomRoute)
	register([]string{"POST"}, PrefixClient, "admin/quarantine_media/:roomId", mxUnstable, router, quarantineRoomRoute) // synapse compat
	register([]string{"POST"}, PrefixMedia, "admin/quarantine/user/:userId", mxUnstable, router, makeRoute(_routers.RequireAccessToken(custom.QuarantineUserMedia), "quarantine_user", counter))
	register([]string{"POST"}, PrefixMedia, "admin/quarantine/server/:serverName", mxUnstable, router, makeRoute(_routers.RequireAccessToken(custom.QuarantineDomainMedia), "quarantine_domain", counter))
	register([]string{"POST"}, PrefixMedia, "admin/quarantine/media/:server/:mediaId", mxUnstable, router, makeRoute(_routers.RequireAccessToken(custom.QuarantineMedia), "quarantine_media", counter))
	register([]string{"GET"}, PrefixMedia, "admin/datastores/:datastoreId/size_estimate", mxUnstable, router, makeRoute(_routers.RequireRepoAdmin(custom.GetDatastoreStorageEstimate), "get_storage_estimate", counter))
	register([]string{"POST"}, PrefixMedia, "admin/datastores/:sourceDsId/transfer_to/:targetDsId", mxUnstable, router, makeRoute(_routers.RequireRepoAdmin(custom.MigrateBetweenDatastores), "datastore_transfer", counter))
	register([]string{"GET"}, PrefixMedia, "admin/datastores", mxUnstable, router, makeRoute(_routers.RequireRepoAdmin(custom.GetDatastores), "list_datastores", counter))
	register([]string{"GET"}, PrefixMedia, "admin/federation/test/:serverName", mxUnstable, router, makeRoute(_routers.RequireRepoAdmin(custom.GetFederationInfo), "federation_test", counter))
	register([]string{"GET"}, PrefixMedia, "admin/usage/:serverName", mxUnstable, router, makeRoute(_routers.RequireRepoAdmin(custom.GetDomainUsage), "domain_usage", counter))
	register([]string{"GET"}, PrefixMedia, "admin/usage/:serverName/users", mxUnstable, router, makeRoute(_routers.RequireRepoAdmin(custom.GetUserUsage), "user_usage", counter))
	register([]string{"GET"}, PrefixMedia, "admin/usage/:serverName/users-stats", mxUnstable, router, makeRoute(_routers.RequireAccessToken(custom.GetUsersUsageStats), "users_usage_stats", counter))
	register([]string{"GET"}, PrefixMedia, "admin/usage/:serverName/uploads", mxUnstable, router, makeRoute(_routers.RequireRepoAdmin(custom.GetUploadsUsage), "uploads_usage", counter))
	register([]string{"GET"}, PrefixMedia, "admin/task/:taskId", mxUnstable, router, makeRoute(_routers.RequireRepoAdmin(custom.GetTask), "get_background_task", counter))
	register([]string{"GET"}, PrefixMedia, "admin/tasks/all", mxUnstable, router, makeRoute(_routers.RequireRepoAdmin(custom.ListAllTasks), "list_all_background_tasks", counter))
	register([]string{"GET"}, PrefixMedia, "admin/tasks/unfinished", mxUnstable, router, makeRoute(_routers.RequireRepoAdmin(custom.ListUnfinishedTasks), "list_unfinished_background_tasks", counter))
	register([]string{"POST"}, PrefixMedia, "admin/user/:userId/export", mxUnstable, router, makeRoute(_routers.RequireAccessToken(custom.ExportUserData), "export_user_data", counter))
	register([]string{"POST"}, PrefixMedia, "admin/server/:serverName/export", mxUnstable, router, makeRoute(_routers.RequireAccessToken(custom.ExportServerData), "export_server_data", counter))
	register([]string{"GET"}, PrefixMedia, "admin/export/:exportId/view", mxUnstable, router, makeRoute(_routers.OptionalAccessToken(custom.ViewExport), "view_export", counter))
	register([]string{"GET"}, PrefixMedia, "admin/export/:exportId/metadata", mxUnstable, router, makeRoute(_routers.OptionalAccessToken(custom.GetExportMetadata), "get_export_metadata", counter))
	register([]string{"GET"}, PrefixMedia, "admin/export/:exportId/part/:partId", mxUnstable, router, makeRoute(_routers.OptionalAccessToken(custom.DownloadExportPart), "download_export_part", counter))
	register([]string{"DELETE"}, PrefixMedia, "admin/export/:exportId/delete", mxUnstable, router, makeRoute(_routers.OptionalAccessToken(custom.DeleteExport), "delete_export", counter))
	register([]string{"POST"}, PrefixMedia, "admin/import", mxUnstable, router, makeRoute(_routers.RequireRepoAdmin(custom.StartImport), "start_import", counter))
	register([]string{"POST"}, PrefixMedia, "admin/import/:importId/part", mxUnstable, router, makeRoute(_routers.RequireRepoAdmin(custom.AppendToImport), "append_to_import", counter))
	register([]string{"POST"}, PrefixMedia, "admin/import/:importId/close", mxUnstable, router, makeRoute(_routers.RequireRepoAdmin(custom.StopImport), "stop_import", counter))
	register([]string{"GET"}, PrefixMedia, "admin/media/:server/:mediaId/attributes", mxUnstable, router, makeRoute(_routers.RequireAccessToken(custom.GetAttributes), "get_media_attributes", counter))
	register([]string{"POST"}, PrefixMedia, "admin/media/:server/:mediaId/attributes", mxUnstable, router, makeRoute(_routers.RequireAccessToken(custom.SetAttributes), "set_media_attributes", counter))

	return router
}

func makeRoute(generator _routers.GeneratorFn, name string, counter *_routers.RequestCounter) http.Handler {
	return _routers.NewInstallMetadataRouter(name == "healthz", name, counter,
		_routers.NewInstallHeadersRouter(
			_routers.NewHostRouter(
				_routers.NewMetricsRequestRouter(
					_routers.NewRContextRouter(generator, _routers.NewMetricsResponseRouter(nil)),
				),
			),
		))
}

type matrixVersions []string

var (
	//mxAllSpec            matrixVersions = []string{"r0", "v1", "v3", "unstable", "unstable/io.t2bot.media"}
	mxUnstable           matrixVersions = []string{"unstable", "unstable/io.t2bot.media"}
	mxSpecV3Transition   matrixVersions = []string{"r0", "v1", "v3"}
	mxSpecV3TransitionCS matrixVersions = []string{"r0", "v3"}
	mxR0                 matrixVersions = []string{"r0"}
	mxV1                 matrixVersions = []string{"v1"}
	mxV3                 matrixVersions = []string{"v3"}
)

func register(methods []string, prefix string, postfix string, versions matrixVersions, router *httprouter.Router, handler http.Handler) {
	for _, method := range methods {
		for _, version := range versions {
			path := fmt.Sprintf("%s/%s/%s", prefix, version, postfix)
			router.Handler(method, path, handler)
			logrus.Debug("Registering route: ", method, path)
		}
	}
}
