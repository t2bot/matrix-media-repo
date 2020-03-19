# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to 
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
* Fixed the cache metrics reporting inaccurate values.

### Changed

* Updated to Go 1.14

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

[unreleased]: https://github.com/turt2live/matrix-media-repo/compare/v1.0.2...HEAD
[1.0.2]: https://github.com/turt2live/matrix-media-repo/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/turt2live/matrix-media-repo/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/turt2live/matrix-media-repo/compare/v1.0.0-rc.2...v1.0.0
[1.0.0-rc.2]: https://github.com/turt2live/matrix-media-repo/compare/v1.0.0-rc.1...v1.0.0-rc.2
[1.0.0-rc.1]: https://github.com/turt2live/matrix-media-repo/releases/tag/v1.0.0-rc.1
