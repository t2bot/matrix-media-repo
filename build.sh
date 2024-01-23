#!/bin/sh

set -ex

GOBIN=$PWD/bin go install -v ./cmd/utilities/compile_assets
$PWD/bin/compile_assets
GOBIN=$PWD/bin go install -ldflags "-X github.com/t2bot/matrix-media-repo/common/version.Version=$(git describe --tags)" -v ./cmd/...
GOBIN=$PWD/bin go install -pgo=pgo_media_repo.pprof -ldflags "-X github.com/t2bot/matrix-media-repo/common/version.Version=$(git describe --tags)" -v ./cmd/workers/media_repo
