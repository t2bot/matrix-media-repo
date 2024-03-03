package main

import (
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/cmd/homeserver_live_importers/internal"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/homeserver_interop/synapse"
)

func main() {
	cfg := internal.InitImportPsqlMatrixDownload("Synapse")
	ctx := rcontext.Initial()

	ctx.Log.Debug("Connecting to homeserver database...")
	hsDb, err := synapse.OpenDatabase(cfg.ConnectionString)
	if err != nil {
		logrus.Fatalf("Failed to open database: %v", err)
	}

	internal.PsqlMatrixDownloadCopy[synapse.LocalMedia](ctx, cfg, hsDb, func(record *synapse.LocalMedia) (*internal.MediaMetadata, error) {
		return &internal.MediaMetadata{
			MediaId:        record.MediaId,
			ContentType:    record.ContentType,
			FileName:       record.UploadName,
			UploaderUserId: record.UserId,
			SizeBytes:      record.SizeBytes,
		}, nil
	})
}
