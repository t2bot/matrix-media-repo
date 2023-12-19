package main

import (
	"io"
	"os"
	"path"

	"github.com/turt2live/matrix-media-repo/archival/v2archive"
	"github.com/turt2live/matrix-media-repo/cmd/homeserver_offline_exporters/_common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/homeserver_interop/synapse"
	"github.com/turt2live/matrix-media-repo/util"
)

func main() {
	cfg := _common.InitExportPsqlFlatFile("Synapse", "media_store_path")
	ctx := rcontext.InitialNoConfig()

	ctx.Log.Debug("Connecting to homeserver database...")
	hsDb, err := synapse.OpenDatabase(cfg.ConnectionString)
	if err != nil {
		panic(err)
	}

	err = _common.ProcessArchiveDirectory(ctx, cfg.ServerName, cfg.SourcePath, func(record *v2archive.ManifestRecord, f io.ReadCloser) error {
		defer f.Close()
		mxc := util.MxcUri(record.Origin, record.MediaId)

		if ok, err := hsDb.HasMedia(record.MediaId); err != nil {
			return err
		} else if ok {
			ctx.Log.Infof("%s has already been exported to Synapse", mxc)
			return nil
		}

		// For MediaID AABBCCDD :
		// $exportPath/local_content/AA/BB/CCDD

		ctx.Log.Infof("Copying %s", mxc)
		directories := path.Join(cfg.ExportPath, "local_content", record.MediaId[0:2], record.MediaId[2:4])
		err = os.MkdirAll(directories, 0655)
		if err != nil {
			return err
		}
		filePath := path.Join(directories, record.MediaId[4:])
		file, err := os.Create(filePath)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(file, f)
		if err != nil {
			return err
		}

		if err = hsDb.InsertMedia(record.MediaId, record.ContentType, record.SizeBytes, record.CreatedTs, record.FileName, record.Uploader); err != nil {
			return err
		}

		// TODO: Populate thumbnails
	})
	if err != nil {
		panic(err)
	}
}
