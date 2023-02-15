package api

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api/_routers"
	"github.com/turt2live/matrix-media-repo/api/custom"
	"github.com/turt2live/matrix-media-repo/api/r0"
	"github.com/turt2live/matrix-media-repo/api/unstable"
	"github.com/turt2live/matrix-media-repo/api/webserver/debug"
)

const PrefixMedia = "/_matrix/media"
const PrefixClient = "/_matrix/client"

func buildRoutes() http.Handler {
	counter := &_routers.RequestCounter{}
	router := buildPrimaryRouter()

	pprofSecret := os.Getenv("MEDIA_PPROF_SECRET_KEY")
	if pprofSecret != "" {
		logrus.Warn("Enabling pprof/debug http endpoints")
		debug.BindPprofEndpoints(router, pprofSecret)
	}

	// Standard (spec) features
	register([]string{"POST"}, PrefixMedia, "upload", false, router, makeRoute(_routers.RequireAccessToken(r0.UploadMedia), "upload", false, counter))
	downloadRoute := makeRoute(_routers.OptionalAccessToken(r0.DownloadMedia), "download", false, counter)
	register([]string{"GET"}, PrefixMedia, "download/:server/:mediaId/:filename", false, router, downloadRoute)
	register([]string{"GET"}, PrefixMedia, "download/:server/:mediaId", false, router, downloadRoute)
	register([]string{"GET"}, PrefixMedia, "thumbnail/:server/:mediaId", false, router, makeRoute(_routers.OptionalAccessToken(r0.ThumbnailMedia), "thumbnail", false, counter))
	register([]string{"GET"}, PrefixMedia, "preview_url", false, router, makeRoute(_routers.RequireAccessToken(r0.PreviewUrl), "url_preview", false, counter))
	register([]string{"GET"}, PrefixMedia, "identicon/*seed", false, router, makeRoute(_routers.OptionalAccessToken(r0.Identicon), "identicon", false, counter))
	register([]string{"GET"}, PrefixMedia, "config", false, router, makeRoute(_routers.RequireAccessToken(r0.PublicConfig), "config", false, counter))
	register([]string{"POST"}, PrefixClient, "logout", false, router, makeRoute(_routers.RequireAccessToken(r0.Logout), "logout", false, counter))
	register([]string{"POST"}, PrefixClient, "logout/all", false, router, makeRoute(_routers.RequireAccessToken(r0.LogoutAll), "logout_all", false, counter))

	// Custom features
	register([]string{"GET"}, PrefixMedia, "local_copy/:server/:mediaId", true, router, makeRoute(_routers.RequireAccessToken(unstable.LocalCopy), "local_copy", false, counter))
	register([]string{"GET"}, PrefixMedia, "info/:server/:mediaId", true, router, makeRoute(_routers.RequireAccessToken(unstable.MediaInfo), "info", false, counter))
	purgeOneRoute := makeRoute(_routers.RequireAccessToken(custom.PurgeIndividualRecord), "purge_individual_media", false, counter)
	register([]string{"DELETE"}, PrefixMedia, "download/:server/:mediaId", true, router, purgeOneRoute)

	// Custom and top-level features
	router.Handler("GET", fmt.Sprintf("%s/version", PrefixMedia), makeRoute(_routers.OptionalAccessToken(custom.GetVersion), "get_version", false, counter))
	healthzRoute := makeRoute(_routers.OptionalAccessToken(custom.GetHealthz), "healthz", true, counter)
	router.Handler("GET", "/healthz", healthzRoute)
	router.Handler("HEAD", "/healthz", healthzRoute)

	// All admin routes are unstable only
	purgeRemoteRoute := makeRoute(_routers.RequireRepoAdmin(custom.PurgeRemoteMedia), "purge_remote_media", false, counter)
	register([]string{"POST"}, PrefixMedia, "admin/purge_remote", true, router, purgeRemoteRoute)
	register([]string{"POST"}, PrefixMedia, "admin/purge/remote", true, router, purgeRemoteRoute)
	register([]string{"POST"}, PrefixClient, "admin/purge_media_cache", true, router, purgeRemoteRoute) // synapse compat
	register([]string{"POST"}, PrefixMedia, "admin/purge/media/:server/:mediaId", true, router, purgeOneRoute)
	register([]string{"POST"}, PrefixMedia, "admin/purge/old", true, router, makeRoute(_routers.RequireRepoAdmin(custom.PurgeOldMedia), "purge_old_media", false, counter))
	register([]string{"POST"}, PrefixMedia, "admin/purge/quarantined", true, router, makeRoute(_routers.RequireAccessToken(custom.PurgeQuarantined), "purge_quarantined", false, counter))
	register([]string{"POST"}, PrefixMedia, "admin/purge/user/:userId", true, router, makeRoute(_routers.RequireAccessToken(custom.PurgeUserMedia), "purge_user_media", false, counter))
	register([]string{"POST"}, PrefixMedia, "admin/purge/room/:roomId", true, router, makeRoute(_routers.RequireAccessToken(custom.PurgeRoomMedia), "purge_room_media", false, counter))
	register([]string{"POST"}, PrefixMedia, "admin/purge/server/:serverName", true, router, makeRoute(_routers.RequireAccessToken(custom.PurgeDomainMedia), "purge_domain_media", false, counter))
	quarantineRoomRoute := makeRoute(_routers.RequireAccessToken(custom.QuarantineRoomMedia), "quarantine_room", false, counter)
	register([]string{"POST"}, PrefixMedia, "admin/quarantine/room/:roomId", true, router, quarantineRoomRoute)
	register([]string{"POST"}, PrefixClient, "admin/quarantine_media/:roomId", true, router, quarantineRoomRoute) // synapse compat
	register([]string{"POST"}, PrefixMedia, "admin/quarantine/user/:userId", true, router, makeRoute(_routers.RequireAccessToken(custom.QuarantineUserMedia), "quarantine_user", false, counter))
	register([]string{"POST"}, PrefixMedia, "admin/quarantine/server/:serverName", true, router, makeRoute(_routers.RequireAccessToken(custom.QuarantineDomainMedia), "quarantine_domain", false, counter))
	register([]string{"POST"}, PrefixMedia, "admin/quarantine/media/:server/:mediaId", true, router, makeRoute(_routers.RequireAccessToken(custom.QuarantineMedia), "quarantine_media", false, counter))
	register([]string{"GET"}, PrefixMedia, "admin/datastores/:datastoreId/size_estimate", true, router, makeRoute(_routers.RequireRepoAdmin(custom.GetDatastoreStorageEstimate), "get_storage_estimate", false, counter))
	register([]string{"POST"}, PrefixMedia, "admin/datastores/:sourceDsId/transfer_to/:targetDsId", true, router, makeRoute(_routers.RequireRepoAdmin(custom.MigrateBetweenDatastores), "datastore_transfer", false, counter))
	register([]string{"GET"}, PrefixMedia, "admin/datastores", true, router, makeRoute(_routers.RequireRepoAdmin(custom.GetDatastores), "list_datastores", false, counter))
	register([]string{"GET"}, PrefixMedia, "admin/federation/test/:serverName", true, router, makeRoute(_routers.RequireRepoAdmin(custom.GetFederationInfo), "federation_test", false, counter))
	register([]string{"GET"}, PrefixMedia, "admin/usage/:serverName", true, router, makeRoute(_routers.RequireRepoAdmin(custom.GetDomainUsage), "domain_usage", false, counter))
	register([]string{"GET"}, PrefixMedia, "admin/usage/:serverName/users", true, router, makeRoute(_routers.RequireRepoAdmin(custom.GetUserUsage), "user_usage", false, counter))
	register([]string{"GET"}, PrefixMedia, "admin/usage/:serverName/users-stats", true, router, makeRoute(_routers.RequireAccessToken(custom.GetUsersUsageStats), "users_usage_stats", false, counter))
	register([]string{"GET"}, PrefixMedia, "admin/usage/:serverName/uploads", true, router, makeRoute(_routers.RequireRepoAdmin(custom.GetUploadsUsage), "uploads_usage", false, counter))
	register([]string{"GET"}, PrefixMedia, "admin/task/:taskId", true, router, makeRoute(_routers.RequireRepoAdmin(custom.GetTask), "get_background_task", false, counter))
	register([]string{"GET"}, PrefixMedia, "admin/tasks/all", true, router, makeRoute(_routers.RequireRepoAdmin(custom.ListAllTasks), "list_all_background_tasks", false, counter))
	register([]string{"GET"}, PrefixMedia, "admin/tasks/unfinished", true, router, makeRoute(_routers.RequireRepoAdmin(custom.ListUnfinishedTasks), "list_unfinished_background_tasks", false, counter))
	register([]string{"POST"}, PrefixMedia, "admin/user/:userId/export", true, router, makeRoute(_routers.RequireAccessToken(custom.ExportUserData), "export_user_data", false, counter))
	register([]string{"POST"}, PrefixMedia, "admin/server/:serverName/export", true, router, makeRoute(_routers.RequireAccessToken(custom.ExportServerData), "export_server_data", false, counter))
	register([]string{"GET"}, PrefixMedia, "admin/export/:exportId/view", true, router, makeRoute(_routers.OptionalAccessToken(custom.ViewExport), "view_export", false, counter))
	register([]string{"GET"}, PrefixMedia, "admin/export/:exportId/metadata", true, router, makeRoute(_routers.OptionalAccessToken(custom.GetExportMetadata), "get_export_metadata", false, counter))
	register([]string{"GET"}, PrefixMedia, "admin/export/:exportId/part/:partId", true, router, makeRoute(_routers.OptionalAccessToken(custom.DownloadExportPart), "download_export_part", false, counter))
	register([]string{"DELETE"}, PrefixMedia, "admin/export/:exportId/delete", true, router, makeRoute(_routers.OptionalAccessToken(custom.DeleteExport), "delete_export", false, counter))
	register([]string{"POST"}, PrefixMedia, "admin/import", true, router, makeRoute(_routers.RequireRepoAdmin(custom.StartImport), "start_import", false, counter))
	register([]string{"POST"}, PrefixMedia, "admin/import/:importId/part", true, router, makeRoute(_routers.RequireRepoAdmin(custom.AppendToImport), "append_to_import", false, counter))
	register([]string{"POST"}, PrefixMedia, "admin/import/:importId/close", true, router, makeRoute(_routers.RequireRepoAdmin(custom.StopImport), "stop_import", false, counter))
	register([]string{"GET"}, PrefixMedia, "admin/media/:server/:mediaId/attributes", true, router, makeRoute(_routers.RequireAccessToken(custom.GetAttributes), "get_media_attributes", false, counter))
	register([]string{"POST"}, PrefixMedia, "admin/media/:server/:mediaId/attributes", true, router, makeRoute(_routers.RequireAccessToken(custom.SetAttributes), "set_media_attributes", false, counter))

	// TODO: Register pprof

	return router
}

func makeRoute(generator _routers.GeneratorFn, name string, ignoreHost bool, counter *_routers.RequestCounter) http.Handler {
	return _routers.NewInstallMetadataRouter(ignoreHost, name, counter,
		_routers.NewInstallHeadersRouter(
			_routers.NewHostRouter(
				_routers.NewMetricsRequestRouter(
					_routers.NewRContextRouter(generator, _routers.NewMetricsResponseRouter(nil)),
				),
			),
		))
}

var versions = []string{"r0", "v1", "v3", "unstable", "unstable/io.t2bot.media"}

func register(methods []string, prefix string, postfix string, unstableOnly bool, router *httprouter.Router, handler http.Handler) {
	for _, method := range methods {
		for _, version := range versions {
			if unstableOnly && !strings.HasPrefix(version, "unstable") {
				continue
			}
			path := fmt.Sprintf("%s/%s/%s", prefix, version, postfix)
			router.Handler(method, path, handler)
			logrus.Debug("Registering route: ", method, path)
		}
	}
}
