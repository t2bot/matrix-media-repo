package main

import (
	"github.com/turt2live/matrix-media-repo/cmd/homeserver_live_importers/_common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/homeserver_interop/dendrite"
)

func main() {
	cfg := _common.InitImportPsqlMatrixDownload("Dendrite")
	ctx := rcontext.InitialNoConfig()

	ctx.Log.Debug("Connecting to homeserver database...")
	hsDb, err := dendrite.OpenDatabase(cfg.ConnectionString, cfg.ServerName)
	if err != nil {
		panic(err)
	}

	_common.PsqlMatrixDownloadCopy[dendrite.LocalMedia](ctx, cfg, hsDb, func(record *dendrite.LocalMedia) (*_common.MediaMetadata, error) {
		return &_common.MediaMetadata{
			MediaId:        record.MediaId,
			ContentType:    record.ContentType,
			FileName:       record.UploadName,
			UploaderUserId: record.UserId,
			SizeBytes:      record.FileSizeBytes,
		}, nil
	})
}
