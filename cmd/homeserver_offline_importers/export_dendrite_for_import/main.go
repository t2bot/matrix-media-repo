package main

import (
	"io"
	"os"
	"path"

	"github.com/t2bot/matrix-media-repo/archival/v2archive"
	"github.com/t2bot/matrix-media-repo/cmd/homeserver_offline_importers/internal"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/homeserver_interop/dendrite"
	"github.com/t2bot/matrix-media-repo/util"
)

func main() {
	cfg := internal.InitExportPsqlFlatFile("Dendrite", "media_api.base_path")
	ctx := rcontext.InitialNoConfig()

	ctx.Log.Debug("Connecting to homeserver database...")
	hsDb, err := dendrite.OpenDatabase(cfg.ConnectionString, cfg.ServerName)
	if err != nil {
		panic(err)
	}

	internal.PsqlFlatFileArchive[dendrite.LocalMedia](ctx, cfg, hsDb, func(r *dendrite.LocalMedia) (v2archive.MediaInfo, io.ReadCloser, error) {
		// For Base64Hash ABCCDD :
		// $importPath/A/B/CCDD/file

		mxc := util.MxcUri(cfg.ServerName, r.MediaId)

		ctx.Log.Info("Copying " + mxc)

		filePath := path.Join(cfg.ImportPath, r.Base64Hash[0:1], r.Base64Hash[1:2], r.Base64Hash[2:], "file")

		f, err := os.Open(filePath)
		if os.IsNotExist(err) && cfg.SkipMissing {
			ctx.Log.Warn("File does not appear to exist, skipping: " + filePath)
			return v2archive.MediaInfo{
				FileName: filePath,
			}, nil, err
		}
		if err != nil {
			return v2archive.MediaInfo{}, nil, err
		}

		return v2archive.MediaInfo{
			Origin:      cfg.ServerName,
			MediaId:     r.MediaId,
			FileName:    r.UploadName,
			ContentType: r.ContentType,
			CreationTs:  r.CreationTs,
			S3Url:       "",
			UserId:      r.UserId,
		}, f, nil
	})
}
