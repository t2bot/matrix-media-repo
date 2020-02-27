# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to 
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

*Nothing yet*.

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

[unreleased]: https://github.com/turt2live/matrix-media-repo/compare/v1.0.1...HEAD
[1.0.1]: https://github.com/turt2live/matrix-media-repo/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/turt2live/matrix-media-repo/compare/v1.0.0-rc.2...v1.0.0
[1.0.0-rc.2]: https://github.com/turt2live/matrix-media-repo/compare/v1.0.0-rc.1...v1.0.0-rc.2
[1.0.0-rc.1]: https://github.com/turt2live/matrix-media-repo/releases/tag/v1.0.0-rc.1
