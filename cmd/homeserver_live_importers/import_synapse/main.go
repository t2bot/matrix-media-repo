package main

import (
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/cmd/homeserver_live_importers/_common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/homeserver_interop/synapse"
)

func main() {
	cfg := _common.InitImportPsqlMatrixDownload("Synapse")
	ctx := rcontext.Initial()

	ctx.Log.Debug("Connecting to homeserver database...")
	hsDb, err := synapse.OpenDatabase(cfg.ConnectionString)
	if err != nil {
		logrus.Fatalf("Failed to open database: %v", err)
	}

	_common.PsqlMatrixDownloadCopy[synapse.LocalMedia](ctx, cfg, hsDb, func(record *synapse.LocalMedia) (*_common.MediaMetadata, error) {
		return &_common.MediaMetadata{
			MediaId:        record.MediaId,
			ContentType:    record.ContentType,
			FileName:       record.UploadName,
			UploaderUserId: record.UserId,
			SizeBytes:      record.SizeBytes,
		}, nil
	})
}
