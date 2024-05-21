package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"sync"

	"github.com/disintegration/imaging"
	"github.com/t2bot/matrix-media-repo/archival/v2archive"
	"github.com/t2bot/matrix-media-repo/cmd/homeserver_offline_exporters/_common"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/homeserver_interop/synapse"
	"github.com/t2bot/matrix-media-repo/thumbnailing"
	"github.com/t2bot/matrix-media-repo/util"
)

type thumbnailSize struct {
	width  int
	height int
	method string
}

var synapseDefaultSizes = []thumbnailSize{
	{32, 32, "crop"},
	{96, 96, "crop"},
	{320, 240, "scale"},
	{640, 480, "scale"},
	{800, 600, "scale"},
}

func main() {
	cfg := _common.InitExportPsqlFlatFile("Synapse", "media_store_path")
	ctx := rcontext.InitialNoConfig()
	ctx.Config.Thumbnails = config.ThumbnailsConfig{
		Types: []string{
			"image/jpeg",
			"image/jpg",
			"image/png",
			"image/apng",
			"image/gif",
			"image/heif",
			"image/heic",
			"image/webp",
			"image/bmp",
			"image/tiff",
		},
		MaxPixels:      32000000, // 32M
		MaxSourceBytes: 10485760, // 10MB
	}

	ctx.Log.Debug("Connecting to homeserver database...")
	hsDb, err := synapse.OpenDatabase(cfg.ConnectionString)
	if err != nil {
		panic(err)
	}

	err = _common.ProcessArchiveDirectory(ctx, cfg.ServerName, cfg.SourcePath, func(record *v2archive.ManifestRecord, f io.ReadCloser) error {
		defer f.Close()
		mxc := util.MxcUri(record.Origin, record.MediaId)

		if record.SizeBytes > math.MaxInt32 {
			ctx.Log.Warnf("%s is potentially too large for Synapse to handle. See https://github.com/matrix-org/synapse/issues/12023 for details.", mxc)
		}

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
		err = os.MkdirAll(directories, 0755)
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

		if !cfg.GenerateThumbnails {
			return nil // we're done here
		}

		// Try to generate thumbnails concurrently
		ctx.Log.Infof("Generating thumbnails for %s", mxc)
		thumbWg := &sync.WaitGroup{}
		dirLock := &sync.Mutex{}
		thumbWg.Add(len(synapseDefaultSizes))
		for _, size := range synapseDefaultSizes {
			go func(s thumbnailSize) {
				defer thumbWg.Done()

				src, err := os.Open(filePath)
				if err != nil {
					ctx.Log.Warn("Error preparing thumbnail. ", s, err)
					return
				}
				defer src.Close()

				thumb, err := thumbnailing.GenerateThumbnail(src, record.ContentType, s.width, s.height, s.method, false, ctx)
				if err != nil {
					if thumb.Reader != nil {
						err2 := thumb.Reader.Close()
						if err2 != nil {
							ctx.Log.Warn("Non-fatal error cleaning up thumbnail stream: ", err2)
						}
					}
					ctx.Log.Debug("Error generating thumbnail (you can probably ignore this). ", s, err)
					return
				}

				img, err := imaging.Decode(thumb.Reader)
				if err != nil {
					ctx.Log.Warn("Error processing thumbnail. ", s, err)
					return
				}

				// Same as media, but different directory
				// $exportPath/local_thumbnails/AA/BB/CCDD/$thumbFile
				dirLock.Lock()
				defer dirLock.Unlock()
				thumbDir := path.Join(cfg.ExportPath, "local_thumbnails", record.MediaId[0:2], record.MediaId[2:4], record.MediaId[4:])
				err = os.MkdirAll(thumbDir, 0755)
				if err != nil {
					ctx.Log.Warn("Error creating thumbnail directories. ", s, err)
					return
				}
				thumbFile, err := os.Create(path.Join(thumbDir, fmt.Sprintf("%d-%d-image-png-%s", img.Bounds().Max.X, img.Bounds().Max.Y, s.method)))
				if err != nil {
					ctx.Log.Warn("Error creating thumbnail. ", s, err)
					return
				}
				defer thumbFile.Close()
				err = imaging.Encode(thumbFile, img, imaging.PNG)
				if err != nil {
					ctx.Log.Warn("Error writing thumbnail. ", s, err)
					return
				}

				pos, err := thumbFile.Seek(0, io.SeekCurrent)
				if err != nil {
					ctx.Log.Warn("Error seeking within thumbnail. ", s, err)
					return
				}

				if err = hsDb.InsertThumbnail(record.MediaId, img.Bounds().Max.X, img.Bounds().Max.Y, "image/png", s.method, pos); err != nil {
					ctx.Log.Warn("Error recording thumbnail. ", s, err)
					return
				}
			}(size)
		}
		thumbWg.Wait()

		// Done
		return nil
	})
	if err != nil {
		ctx.Log.Fatal(err)
	}

	ctx.Log.Info("Done! If there's no warnings above, you're probably fine.")
}
