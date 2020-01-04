$GitCommit = (git rev-list -1 HEAD)
$Version = (git describe --tags)
go install -v ./cmd/compile_assets
bin\compile_assets.exe
go install -ldflags "-X github.com/turt2live/matrix-media-repo/common/version.GitCommit=$GitCommit -X github.com/turt2live/matrix-media-repo/common/version.Version=$Version" -v ./cmd/...
