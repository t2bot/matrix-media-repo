package internal

import (
	"fmt"
	"io"
	"os"

	"github.com/t2bot/matrix-media-repo/archival"
	"github.com/t2bot/matrix-media-repo/archival/v2archive"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/homeserver_interop"
)

func PsqlFlatFileArchive[M homeserver_interop.ImportDbMedia](ctx rcontext.RequestContext, cfg *ImportOptsPsqlFlatFile, db homeserver_interop.ImportDb[M], processFn func(record *M) (v2archive.MediaInfo, io.ReadCloser, error)) {
	ctx.Log.Info("Fetching all local media records from homeserver...")
	records, err := db.GetAllMedia()
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
		info, f, err := processFn(r)
		if err != nil {
			if os.IsNotExist(err) && cfg.SkipMissing {
				missing = append(missing, info.FileName)
				return
			}
			ctx.Log.Fatal(err)
		}

		_, err = archiver.AppendMedia(f, info)
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
