#!/bin/sh

GOBIN=$PWD/bin go install -v ./cmd/compile_assets
$PWD/bin/compile_assets

rm -rfv $PWD/bin/win
rm -rfv $PWD/bin/linux
mkdir $PWD/bin/win
mkdir $PWD/bin/linux

GOOS=windows GOARCH=amd64 GOBIN=$PWD/bin go build -o $PWD/bin/win -a -ldflags "-X github.com/turt2live/matrix-media-repo/common/version.GitCommit=$(git rev-list -1 HEAD) -X github.com/turt2live/matrix-media-repo/common/version.Version=$(git describe --tags)" -v ./cmd/...
GOOS=linux GOARCH=amd64 GOBIN=$PWD/bin go build -o $PWD/bin/linux -a -ldflags "-X github.com/turt2live/matrix-media-repo/common/version.GitCommit=$(git rev-list -1 HEAD) -X github.com/turt2live/matrix-media-repo/common/version.Version=$(git describe --tags)" -v ./cmd/...

rm -rfv $PWD/bin/dist
mkdir $PWD/bin/dist
cd $PWD/bin
cd win
for file in * ; do mv -v $file ../dist/${file%.*}-win-x64.exe; done;
cd ../linux
for file in * ; do mv -v $file ../dist/${file}-linux-x64; done;
cd ../../

rm -rfv $PWD/bin/win
rm -rfv $PWD/bin/linux

rm -rfv $PWD/bin/dist/compile_assets*
rm -rfv $PWD/bin/dist/loadtest*
rm -rfv $PWD/bin/dist/complement*
