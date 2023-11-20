package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/turt2live/matrix-media-repo/archival"
	"github.com/turt2live/matrix-media-repo/archival/v2archive"
	"github.com/turt2live/matrix-media-repo/cmd/homeserver_offline_importers/_common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/homeserver_interop/synapse"
	"github.com/turt2live/matrix-media-repo/util"
)

func main() {
	cfg := _common.InitExportPsqlFlatFile("Synapse", "media_store_path")
	ctx := rcontext.InitialNoConfig()

	ctx.Log.Debug("Connecting to synapse database...")
	synDb, err := synapse.OpenDatabase(cfg.ConnectionString)
	if err != nil {
		panic(err)
	}

	ctx.Log.Info("Fetching all local media records from Synapse...")
	records, err := synDb.GetAllMedia()
	if err != nil {
		panic(err)
	}

	ctx.Log.Info(fmt.Sprintf("Exporting %d media records", len(records)))

	archiver, err := v2archive.NewWriter(ctx, "OOB", cfg.ServerName, cfg.PartSizeBytes, archival.PersistPartsToDirectory(cfg.ExportPath))
	if err != nil {
		ctx.Log.Fatal(err)
	}

	missing := make([]string, 0)

	for _, r := range records {
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
			missing = append(missing, filePath)
			continue
		}
		if err != nil {
			ctx.Log.Fatal(err)
		}

		_, err = archiver.AppendMedia(f, v2archive.MediaInfo{
			Origin:      cfg.ServerName,
			MediaId:     r.MediaId,
			FileName:    r.UploadName,
			ContentType: r.ContentType,
			CreationTs:  r.CreatedTs,
			S3Url:       "",
			UserId:      r.UserId,
		})
		if err != nil {
			ctx.Log.Fatal(err)
		}
	}

	err = archiver.Finish()
	if err != nil {
		ctx.Log.Fatal(err)
	}

	ctx.Log.Info("Done export")

	// Report missing files
	if len(missing) > 0 {
		for _, m := range missing {
			ctx.Log.Warn("Was not able to find " + m)
		}
	}

	ctx.Log.Info("Export completed")
}
