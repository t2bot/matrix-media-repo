# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to 
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

* Allow guests to access uploaded media, as per [MSC4189](https://github.com/matrix-org/matrix-spec-proposals/pull/4189).

### Changed

* MMR now requires Go 1.22 for compilation.

### Fixed

* Return a 404 instead of 500 when clients access media which is frozen.
* Return a 403 instead of 500 when guests access endpoints that are for registered users only.
* Ensure the request parameters are correctly set for authenticated media client requests.
* Ensure remote signing keys expire after at most 7 days.
* Fixed parsing of `Authorization` headers for federated servers.

## [1.3.7] - July 30, 2024

### Added

* A new global config option, `repo.freezeUnauthenticatedMedia`, is supported to enact the unauthenticated media freeze early. See `config.sample.yaml` for details.

### Changed

* The default leaky bucket capacity has changed from 300mb to 500mb, allowing for more downloads to go through. The drain rate and overflow limit are unchanged (5mb/minute and 100mb respectively).

## [1.3.6] - July 10, 2024

### Fixed

* Ensure a `boundary` is set on federation downloads, allowing the download to work.

## [1.3.5] - July 10, 2024

### Added

* New datastore option to ignore Redis cache when downloading media served by a `publicBaseUrl`. This can help ensure more requests get redirected to the CDN.
* `HEAD /download` is now supported, as per [MSC4120](https://github.com/matrix-org/matrix-spec-proposals/pull/4120).
* S3 datastores can now specify a `prefixLength` to improve S3 performance on some providers. See `config.sample.yaml` for details.
* Add `multipartUploads` flag for running MMR against unsupported S3 providers. See `config.sample.yaml` for details. 
* A new "leaky bucket" rate limit algorithm has been applied to downloads. See `rateLimit.buckets` in `config.sample.yaml` for details.
* Add support for [MSC3916: Authentication for media](https://github.com/matrix-org/matrix-spec-proposals/pull/3916). 
  * To enable full support, use `signingKeyPath` in your config. See `config.sample.yaml` for details. 
  * Server operators should point `/_matrix/client/v1/media/*` and `/_matrix/federation/v1/media/*` at MMR.

### Changed

* The leaky bucket rate limiting introduced above is turned on by default. Administrators are encouraged to review the default settings and adjust as needed.

### Fixed

* Metrics for redirected and HTML requests are tracked.
* Fixed more issues relating to non-dimensional media being thumbnailed (`invalid image size: 0x0` errors).
* Long-running purge requests no longer fail when the requesting client times out. They are continued in the background.
* Purging old media has been fixed to actually identify old media.
* JPEG thumbnails will now use sensible extensions.
* Fixed directory permissions when exporting MMR to Synapse.
* In some rare cases, memory usage may have leaked due to thumbnail error handling. This has been fixed.
* Synapse signing keys with blank lines can now be decoded/combined with other keys.

## [1.3.4] - February 9, 2024

### Added

* Dendrite homeservers can now have their media imported safely, and `adminApiKind` may be set to `dendrite`.
* Exporting MMR's data to Synapse is now possible with `import_to_synapse`. To use it, first run `gdpr_export` or similar.
* Errors encountered during a background task, such as an API-induced export, are exposed as `error_message` in the admin API.
* MMR will follow redirects on federated downloads up to 5 hops.
* S3-backed datastores can have download requests redirected to a public-facing CDN rather than being proxied through MMR. See `publicBaseUrl` under the S3 datastore config.

### Changed

* Exports now use an internal timeout of 10 minutes instead of 1 minute when downloading files. This may still result in errors if downloading from S3 takes too long.
* MMR now requires Go 1.21 for compilation.
* ARM-supported Docker images are now available through [GHCR](https://github.com/t2bot/matrix-media-repo/pkgs/container/matrix-media-repo).
  * The Docker Hub (docker.io) builds are deprecated and will not receive updates starting with v1.4.0
  * Docker Hub images are not guaranteed to have ARM compatibility.
* The `latest` Docker tag on both Docker Hub and GHCR now points to the latest release instead of the unstable development build.

### Fixed

* Exports created with `s3_urls` now contain valid URLs.
* Exports no longer fail with "The requested range is not satisfiable".
* Exports no longer fail with "index out of range \[0] with length 0".
* Requests requiring authentication, but lack a provided access token, will return HTTP 401 instead of HTTP 500 now.
* Downloads when using a self-hosted MinIO instance are no longer slower than expected.
* The `DELETE /_matrix/media/unstable/admin/export/:exportId` endpoint has been reinstated as described.
* If a server's `downloads.maxSize` is greater than the `uploads.maxSize`, remote media is no longer cut off at `uploads.maxSize`. The media will instead be downloaded at `downloads.maxSize` and error if greater.
* `Content-Type` on `/download` and `/thumbnail` is now brought in line with [MSC2701](https://github.com/matrix-org/matrix-spec-proposals/pull/2701).

## [1.3.3] - October 31, 2023

### Fixed

* Improved handling when encountering an error attempting to populate Redis during uploads.
* Fixed `Range` requests failing by default by internally setting a default chunk size of 10mb.
* Stop logging "no exif data".
* Fixed admin API requests not working when authenticating as the shared secret user.

### Changed

* Updated dependencies. Manually compiled deployments may need to recompile `libheif` as well.

## [1.3.2] - September 13, 2023

### Fixed

* Fixed thumbnail generation causing `thumbnails_index` errors in some circumstances.

## [1.3.1] - September 8, 2023

### Fixed

* Fixed media purge API not being able to delete thumbnails.
* Fixed thumbnails being attempted for disabled media types.
* Fixed SVG and other non-dimensional media failing to be usefully thumbnailed in some cases.

## [1.3.0] - September 8, 2023

### Mandatory Configuration Change

**Please see [docs.t2bot.io](https://docs.t2bot.io/matrix-media-repo/v1.3.3/upgrading/130.html) for details.**

### Security Fixes

* Fix improper usage of `Content-Disposition: inline` and related `Content-Type` safety ([CVE-2023-41318](https://www.cve.org/CVERecord?id=CVE-2023-41318), [GHSA-5crw-6j7v-xc72](https://github.com/t2bot/matrix-media-repo/security/advisories/GHSA-5crw-6j7v-xc72)).

### Deprecations

* The `GET /_matrix/media/unstable/local_copy/:server/:mediaId` (and `unstable/io.t2bot.media` variant) endpoint is deprecated and scheduled for removal. If you are using this endpoint, please comment on [this issue](https://github.com/t2bot/matrix-media-repo/issues/422) to explain your use case.

### Added

* Added a `federation.ignoredHosts` config option to block media from individual homeservers.
* Support for [MSC2246](https://github.com/matrix-org/matrix-spec-proposals/pull/2246) (async uploads) is added, with per-user quota limiting options.
* Support for [MSC4034](https://github.com/matrix-org/matrix-spec-proposals/pull/4034) (self-serve usage information) is added, alongside a new "maximum file count" quota limit.
* The `GET /_synapse/admin/v1/statistics/users/media` [endpoint](https://matrix-org.github.io/synapse/v1.88/admin_api/statistics.html#users-media-usage-statistics) from Synapse is now supported at the same path for local server admins.
* Thumbnailing support for:
  * BMP images.
  * TIFF images.
  * HEIC images.
* New metrics:
  * HTTP response times.
  * Age of downloaded/accessed media.
* Support for [PGO](https://go.dev/doc/pgo) builds has been enabled via [pgo-fleet](https://github.com/t2bot/pgo-fleet).

### Removed

* IPFS support has been removed due to maintenance burden.
* Exports initiated through the admin API no longer support `?include_data=false`. Exports will always contain data.
* Server-side blurhash calculation has been removed. Clients and bridges already calculate blurhashes locally where applicable. 

### Changed

* **Mandatory configuration change**: You must add datastore IDs to your datastore configuration, as matrix-media-repo will no longer manage datastores for you.
* If compiling `matrix-media-repo`, note that new external dependencies are required. See [the docs](https://docs.t2bot.io/matrix-media-repo/v1.3.3/installing/method/compilation.html).
  * Docker images already contain these dependencies. 
* Datastores no longer use the `enabled` flag set on them. Use `forKinds: []` instead to disable a datastore's usage.
* Per-user upload quotas now do not allow users to exceed the maximum values, even by 1 byte. Previously, users could exceed the limits by a little bit.
* Updated to Go 1.19, then Go 1.20 in the same release cycle.
* New CGO dependencies are required. See [docs.t2bot.io](https://docs.t2bot.io/matrix-media-repo/v1.3.3/installing/method/compilation.html) for details.
* Logs are now less noisy by default.
* Connected homeservers must support at least Matrix 1.1 on the Client-Server API. Servers over federation are not affected.
* The example Grafana dashboard has been updated.

### Fixed

* URL previews now follow redirects properly.
* Overall memory usage is improved, particularly during media uploads and API-initiated imports.
  * Note: If you use plugins then memory usage will still be somewhat high due to temporary caching of uploads.
  * Note: This affects RSS primarily. VSZ and other memory metrics may be higher than expected due to how Go releases memory to the OS. This is fixed when there's memory pressure.
* Fixed shutdown stall if the config was reloaded more than once while running.

## [1.2.13] - February 12, 2023

### Deprecations

* In version 1.3.0, IPFS will no longer be supported as a datastore. Please migrate your data if you are using the IPFS support.

### Added

* Added the `Cross-Origin-Resource-Policy: cross-origin` header to all downloads, as per [MSC3828](https://github.com/matrix-org/matrix-spec-proposals/pull/3828).
* Added metrics for tracking which S3 operations are performed against datastores.

### Changed

* Swap out the HEIF library for better support towards [ARM64 Docker Images](https://github.com/t2bot/matrix-media-repo/issues/365).
* The development environment now uses Synapse as a homeserver. Test accounts will need recreating.
* Updated to Go 1.18
* Improved error message when thumbnailer cannot determine image dimensions.

### Fixed

* Return default media attributes if none have been explicitly set.

## [1.2.12] - March 31, 2022

### Fixed

* Fixed a permissions check issue on the new statistics endpoint released in v1.2.11

## [1.2.11] - March 31, 2022

### Added

* New config option to set user agent when requesting URL previews.
* Added support for `image/jxl` thumbnailing.
* Built-in early support for content ranges (being able to skip around in audio and video). This is only available if
  caching is enabled.
* New config option for changing the log level.
* New (currently undocumented) binary `s3_consistency_check` to find objects in S3 which *might* not be referenced by
  the media repo database. Note that this can include uploads in progress.
* Admin endpoint to GET users' usage statistics for a server.

### Removed

* Support for the in-memory cache has been removed. Redis or having no cache are now the only options.
* Support for the Redis config under `features` has been removed. It is now only available at the top level of the
  config. See the sample config for more details.

### Fixed

* Fixed media being permanently lost when transferring to an (effectively) readonly S3 datastore.
* Purging non-existent files now won't cause errors.
* Fixed HEIF/HEIC thumbnailing. Note that this thumbnail type might cause increased memory usage.
* Ensure endpoints register in a stable way, making them predictably available.
* Reduced download hits to datastores when using Redis cache.

### Changed

* Updated support for post-[MSC3069](https://github.com/matrix-org/matrix-doc/pull/3069) homeservers.
* Updated the built-in oEmbed `providers.json`

## [1.2.10] - December 23rd, 2021

### Deprecation notices

In a future version (likely the next), the in-memory cache support will be removed. Instead, please use the Redis
caching that is now supported properly by this release, or disable caching if not applicable for your deployment.

### Added

* Added support for setting the Redis database number.

### Fixed

* Fixed an issue with the Redis config not being recognized at the root level.

## [1.2.9] - December 22nd, 2021

### Deprecation notices

In a future version (likely the next), the in-memory cache support will be removed. Instead, please use the Redis
caching that is now supported properly by this release, or disable caching if not applicable for your deployment.

### Added

* Added support for `HEAD` at the `/healthz` endpoint.
* Added `X-Content-Security-Policy: sandbox` in contexts where the normal CSP
  header would be served. This is a limited, pre-standard form of CSP supported
  by IE11, in order to have at least some mitigation of XSS attacks.
* Added support for the `org.matrix.msc2705.animated` query parameter.
* Added support for S3 storage classes (optional).
* Added support for listening on Matrix 1.1 endpoints (`/_matrix/media/v3/*`).

### Changed

* Support the Redis config at the root level of the config, promoting it to a proper feature.

### Fixed

* Improved performance of datastore selection when only one datastore is eligible to contain media.
* Fixed blurhash not enabling itself.
* Fixed blurhash implementation to match MSC.

## [1.2.8] - April 30th, 2021

### Fixed

* Fixed crashes when internal workers encounter panics.

## [1.2.7] - April 19th, 2021

### Security advisories

This release includes a fix for [CVE-2021-29453](https://github.com/t2bot/matrix-media-repo/security/advisories/GHSA-j889-h476-hh9h).

Server administrators are recommended to upgrade as soon as possible. This issue is considered to be exploited in the wild
due to some deployments being affected unexpectedly.

### Added

* Added support for structured logging (JSON).

### Changed

* Turned color-coded logs off by default. This can be changed in the config.

### Fixed

* Fixed memory exhaustion when thumbnailing maliciously crafted images.

## [1.2.6] - March 25th, 2021

### Added

* Added ffmpeg and ImageMagick to Docker image to support specialized thumbnail types.

### Fixed

* Handle guest accounts properly. Previously they were still declined, though by coincidence.

## [1.2.5] - March 17th, 2021

### Added

* Added a `-verify` mode to imports to determine if large imports were successful.
* Added optional support for [Sentry](https://sentry.io/) (error reporting).

### Changed

* `Content-Disposition` of plain text files now defaults to `inline`.

### Fixed

* Fixed rich oEmbed URL previews (Twitter).
* Fixed photo oEmbed URL previews (Giphy).
* Fixed orientation parsing for some thumbnails.
* Fixed file name being incorrect on the first download from remote servers.
* Fixed a download inefficiency where remote downloads could use extra bandwidth.
* Fixed a problem where secondary imports can never finish.
* Fixed imports not handling duplicate media IDs.
* Fixed some database connection errors not being handled correctly.

## [1.2.4] - March 5th, 2021

### Fixed

* Fixed build error for modern versions of Go, improving IPFS implementation.

## [1.2.3] - March 4th, 2021

### Added

* Introduced early plugin support (only for antispam for now).
  * Includes a simple OCR plugin to help mitigate text-based image spam.
* Added an `X-Robots-Tag` header to help prevent indexing. Thanks @jellykells!

### Fixed

* Fixed crash when generating some thumbnails of audio.
* Fixed various artifact problems with APNG and GIF thumbnails. Thanks @Sorunome!
* Fixed a missing "unlimited size" check for thumbnails. Thanks @Sorunome!

## [1.2.2] - December 8th, 2020

### Fixed

* Generate JPEG thumbnails for JPEG for reduced file size. Thanks @Sorunome!
* Strip `charset` parameter off binary media for better compatibility with other homeservers.

## [1.2.1] - October 27th, 2020

### Added

* Added a new tool, `export_synapse_for_import`, which can be used to do an offline import from Synapse.
  * After running this tool, use the `gdpr_import` tool to bring the export into the media repo.
* Added thumbnailing support for some audio waveforms (MP3, WAV, OGG, and FLAC).
* Added audio metadata (duration, etc) to the unstable `/info` endpoint. Aligns with [MSC2380](https://github.com/matrix-org/matrix-doc/pull/2380).
* Added simple thumbnailing for MP4 videos.
* Added an `asAttachment` query parameter to download requests per [MSC2702](https://github.com/matrix-org/matrix-doc/pull/2702).

### Fixed

* Fixed thumbnails for invalid JPEGs.
* Fixed incorrect metrics being published when using the Redis cache.
* Fixed errors generating thumbnails when bad EXIF headers were provided.
* Use `r0` instead of `v1` for federation requests. No changes should be needed to configurations or routing - it'll just work.

## [1.2.0] - August 2nd, 2020

### Upgrade notes

**This release contains a database change which might take a while.** In order to support quotas, this
release tracks how much a user has uploaded, which might take a while to initially calculate. If you have
a large database (more than about 100k uploaded files), run the following steps before upgrading:

1. The PostgreSQL script described [here](https://github.com/t2bot/matrix-media-repo/blob/a8951b0562debb9f8ae3b6e517bfc3a84d2e627a/migrations/17_add_user_stats_table_up.sql).
   This can be run while the server is running.
2. If you have no intention of using stats or quotas, you're done (the stats table will be inaccurate). If
   you do plan on using either, run `INSERT INTO user_stats SELECT user_id, SUM(size_bytes) FROM media GROUP BY user_id;`
   which may take a while.
3. Change the owner of the table and function to your media repo's postgresql user. For example, if your postgres
   user is `media`, then run:
   ```sql
   ALTER TABLE user_stats OWNER TO media;
   ALTER FUNCTION track_update_user_media() OWNER TO media; 
   ```

### Added

* Add webp image support. Thanks @Sorunome!
* Add apng image support. Thanks @Sorunome!
* Experimental support for Redis as a cache (in preparation for proper load balancing/HA support).
* Added oEmbed URL preview support.
* Added support for dynamic thumbnails.
* Added a way to prevent certain media from being quarantined (attributes API).
* Added support for quotas.

### Changed

* Remove deprecated support for restricting uploads to certain mime types.
* Remove deprecated support for `forUploads`.
* Clarified what `uploads.minBytes` is intended to be used for.

### Fixed

* GIFs now thumbnail correctly. Thanks @Sorunome!
* Fixed empty Content-Type header on retrieved remote media. Thanks @silkeh!
* Fixed various issues with IPv6 handling. Thanks @silkeh!
* Fixed high database usage for uploads when only one datastore is present.
* Fixed incorrect HTTP status codes for bad thumbnail requests.
* Fixed dimension checking on thumbnails.
* Fixed handling of EXIF metadata. Thanks @sorunome!
* Fixed handling of URL previews for some encodings.
* Fixed `Cache-Control` headers being present on errors.

## [1.1.3] - July 15th, 2020

### Added

* Added options to cache access tokens for users. This prevents excessive calls to `/account/whoami` on your homeserver, particularly for appservices.
* [Documentation](https://github.com/t2bot/matrix-media-repo/blob/master/docs/contrib/delegation.md) on how to set up delegation with the media repo and Traefik. Thanks @derEisele!

### Changed

* Deprecated support for restricting uploads to certain mime types, due to inability to make it work correctly with encrypted media.
* Removed deprecated `storagePaths` config option. Please use datastores.

### Fixed

* Fixed federation with some homeserver setups (delegation with ports). Thanks @MatMaul!
* Fixed the Synapse import script to not skip duplicated media. Thanks @jaywink!
* Fixed requests to IPv6 hosts. Thanks @MatMaul!
* Removed excessive calls to the database during upload.

## [1.1.2] - April 21st, 2020

### Fixed

* Fixed templates being corrupt in the Docker image.
* Fixed `REPO_CONFIG` environment variable not being respected for auxiliary binaries in the Docker image.

### Changed

* The Docker image now uses the migrations packed into the binary instead of the in-image ones.
* Reduced log spam when someone views an export.

## [1.1.1] - March 26th, 2020

### Added

* Added pprof endpoints for debugging performance. Only enabled with a `MEDIA_PPROF_SECRET_KEY` environment variable.

### Fixed

* Fixed a few very slow memory leaks when using S3 datastores.

## [1.1.0] - March 19th, 2020

### Added

* Added support for [MSC2448](https://github.com/matrix-org/matrix-doc/pull/2448).
* Added support for specifying a `region` to the S3 provider.
* Pass-through the `Accept-Language` header for URL previews, with options to set a default.
* Experimental support for IPFS.
* Consistent inclusion of a charset for certain text `Content-Type`s.
* New metrics for the cache composition reality (`media_cache_num_live_bytes_used` and `media_cache_num_live_items`).

### Fixed

* Fixed thumbnails producing the wrong result.
* Fixed `expireAfterDays` for thumbnails potentially deleting media under some conditions.
* Fixed a bug where items could be double-counted (but not double-stored) in the cache.
* Fixed the cache metrics reporting inaccurate values.
* Fixed a general memory leak in the cache due to inaccurate counting of items in the cache.

### Changed

* Updated to Go 1.14
* Updated the Grafana dashboard and moved it in-tree.

## [1.0.2] - March 3, 2020

### Added

* Added support for a `forKinds: ["all"]` option on datastores.

### Fixed

* Fixed a bug with the cache where it would never expire old entries unless it was pressed for space.
* Fixed a bug with the cache where the minimum cache time trigger would not work.

## [1.0.1] - February 27, 2020

### Fixed

* Fix a memory leak within the cache layers.

## [1.0.0] - January 4, 2020

### Added

* Compile assets (templates and migrations) into the binary for ease of deployment.
* Added binaries to make exports and imports easier.

### Fixed

* Fix error message when an invalid access token is provided.
* Fixed imports not starting in 1.0.0-rc.2.

## [1.0.0-rc.2] - January 3, 2020

### Fixed

* Fixed exports not starting in 1.0.0-rc.1.

## [1.0.0-rc.1] - December 29, 2019

### Added

* First ever release of matrix-media-repo.
* Deduplicate media from all sources.
* Support downloads, thumbnails, URL previews, identicons.
* Support for GDPR-style media exports.
* Support for importing from a previous export (for transferring data between repos).
* Admin utilities for clearing up space and undesirable content.
* Built-in S3 (and S3-like) support.
* Animated thumbnail generation.
* Importing media from an existing Synapse homeserver.
* Support for multiple datastores/locations to store different kinds of media.
* Federation for acquiring remote media.
* Media identification ([MSC2380](https://github.com/matrix-org/matrix-doc/pull/2380)).
* Support for cloning media to the local homeserver.
* Various other features that would be expected like maximum/minimum size controls, rate limiting, etc. Check out the
  sample config for a better idea of what else is possible.

[unreleased]: https://github.com/t2bot/matrix-media-repo/compare/v1.3.7...HEAD
[1.3.7]: https://github.com/t2bot/matrix-media-repo/compare/v1.3.6...v1.3.7
[1.3.6]: https://github.com/t2bot/matrix-media-repo/compare/v1.3.5...v1.3.6
[1.3.5]: https://github.com/t2bot/matrix-media-repo/compare/v1.3.4...v1.3.5
[1.3.4]: https://github.com/t2bot/matrix-media-repo/compare/v1.3.3...v1.3.4
[1.3.3]: https://github.com/t2bot/matrix-media-repo/compare/v1.3.2...v1.3.3
[1.3.2]: https://github.com/t2bot/matrix-media-repo/compare/v1.3.1...v1.3.2
[1.3.1]: https://github.com/t2bot/matrix-media-repo/compare/v1.3.0...v1.3.1
[1.3.0]: https://github.com/t2bot/matrix-media-repo/compare/v1.2.13...v1.3.0
[1.2.13]: https://github.com/t2bot/matrix-media-repo/compare/v1.2.12...v1.2.13
[1.2.12]: https://github.com/t2bot/matrix-media-repo/compare/v1.2.11...v1.2.12
[1.2.11]: https://github.com/t2bot/matrix-media-repo/compare/v1.2.10...v1.2.11
[1.2.10]: https://github.com/t2bot/matrix-media-repo/compare/v1.2.9...v1.2.10
[1.2.9]: https://github.com/t2bot/matrix-media-repo/compare/v1.2.8...v1.2.9
[1.2.8]: https://github.com/t2bot/matrix-media-repo/compare/v1.2.7...v1.2.8
[1.2.6]: https://github.com/t2bot/matrix-media-repo/compare/v1.2.6...v1.2.7
[1.2.6]: https://github.com/t2bot/matrix-media-repo/compare/v1.2.5...v1.2.6
[1.2.5]: https://github.com/t2bot/matrix-media-repo/compare/v1.2.4...v1.2.5
[1.2.4]: https://github.com/t2bot/matrix-media-repo/compare/v1.2.3...v1.2.4
[1.2.3]: https://github.com/t2bot/matrix-media-repo/compare/v1.2.2...v1.2.3
[1.2.2]: https://github.com/t2bot/matrix-media-repo/compare/v1.2.1...v1.2.2
[1.2.1]: https://github.com/t2bot/matrix-media-repo/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/t2bot/matrix-media-repo/compare/v1.1.3...v1.2.0
[1.1.3]: https://github.com/t2bot/matrix-media-repo/compare/v1.1.2...v1.1.3
[1.1.2]: https://github.com/t2bot/matrix-media-repo/compare/v1.1.1...v1.1.2
[1.1.1]: https://github.com/t2bot/matrix-media-repo/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/t2bot/matrix-media-repo/compare/v1.0.2...v1.1.0
[1.0.2]: https://github.com/t2bot/matrix-media-repo/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/t2bot/matrix-media-repo/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/t2bot/matrix-media-repo/compare/v1.0.0-rc.2...v1.0.0
[1.0.0-rc.2]: https://github.com/t2bot/matrix-media-repo/compare/v1.0.0-rc.1...v1.0.0-rc.2
[1.0.0-rc.1]: https://github.com/t2bot/matrix-media-repo/releases/tag/v1.0.0-rc.1
