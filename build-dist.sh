#!/bin/bash

set -ex

rm -rfv $PWD/bin/*
mkdir $PWD/bin/dist

GOBIN=$PWD/bin go install -v ./cmd/compile_assets
$PWD/bin/compile_assets

arches=("amd64")
oses=("windows" "linux")

for os in "${oses[@]}"
do
  for arch in "${arches[@]}"
  do
    pth="$os-$arch"
    mkdir $PWD/bin/$pth
    GOOS=$os GOARCH=$arch GOBIN=$PWD/bin go build -o $PWD/bin/$pth -a -ldflags "-X github.com/turt2live/matrix-media-repo/common/version.GitCommit=$(git rev-list -1 HEAD) -X github.com/turt2live/matrix-media-repo/common/version.Version=$(git describe --tags)" -v ./cmd/...
    GOOS=$os GOARCH=$arch GOBIN=$PWD/bin go build -pgo=pgo_media_repo.pprof -o $PWD/bin/$pth -a -ldflags "-X github.com/turt2live/matrix-media-repo/common/version.GitCommit=$(git rev-list -1 HEAD) -X github.com/turt2live/matrix-media-repo/common/version.Version=$(git describe --tags)" -v ./cmd/media_repo
    cd $PWD/bin/$pth
    if [ "$arch" == "amd64" ]; then
      arch="x64"
    fi
    if [ "$os" == "windows" ]; then
      for file in * ; do mv -v $file ../dist/${file%.*}-win-${arch}.exe; done;
    else
      for file in * ; do mv -v $file ../dist/${file}-${os}-${arch}; done;
    fi
    cd ../../
    rm -rfv $PWD/bin/$pth
  done
done

rm -rfv $PWD/bin/dist/compile_assets*
