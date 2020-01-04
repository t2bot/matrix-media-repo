#!/bin/sh

GOBIN=$PWD/bin go install -v ./cmd/compile_assets
$PWD/bin/compile_assets
GOBIN=$PWD/bin go install -ldflags "-X github.com/turt2live/matrix-media-repo/common/version.GitCommit=$(git rev-list -1 HEAD) -X github.com/turt2live/matrix-media-repo/common/version.Version=$(git describe --tags)" -v ./cmd/...
