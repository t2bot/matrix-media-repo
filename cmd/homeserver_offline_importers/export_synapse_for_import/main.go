package main

import (
	"io"
	"os"
	"path"
	"strings"

	"github.com/t2bot/matrix-media-repo/archival/v2archive"
	"github.com/t2bot/matrix-media-repo/cmd/homeserver_offline_importers/_common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/homeserver_interop/synapse"
	"github.com/t2bot/matrix-media-repo/util"
)

func main() {
	cfg := _common.InitExportPsqlFlatFile("Synapse", "media_store_path")
	ctx := rcontext.InitialNoConfig()

	ctx.Log.Debug("Connecting to homeserver database...")
	hsDb, err := synapse.OpenDatabase(cfg.ConnectionString)
	if err != nil {
		panic(err)
	}

	_common.PsqlFlatFileArchive[synapse.LocalMedia](ctx, cfg, hsDb, func(r *synapse.LocalMedia) (v2archive.MediaInfo, io.ReadCloser, error) {
		// For MediaID AABBCCDD :
		// $importPath/local_content/AA/BB/CCDD
		//
		// For a URL MediaID 2020-08-17_AABBCCDD:
		// $importPath/url_cache/2020-08-17/AABBCCDD

		mxc := util.MxcUri(cfg.ServerName, r.MediaId)

		ctx.Log.Info("Copying " + mxc)

		filePath := path.Join(cfg.ImportPath, "local_content", r.MediaId[0:2], r.MediaId[2:4], r.MediaId[4:])
		if r.UrlCache != "" {
			dateParts := strings.Split(r.MediaId, "_")
			filePath = path.Join(cfg.ImportPath, "url_cache", dateParts[0], strings.Join(dateParts[1:], "_"))
		}

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
			CreationTs:  r.CreatedTs,
			S3Url:       "",
			UserId:      r.UserId,
		}, f, nil
	})
}
