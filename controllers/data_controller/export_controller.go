package data_controller

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/util/ids"
	"io"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/storage/datastore/ds_s3"
	"github.com/turt2live/matrix-media-repo/templating"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
)

type ManifestRecord struct {
	FileName     string `json:"name"`
	ArchivedName string `json:"file_name"`
	SizeBytes    int64  `json:"size_bytes"`
	ContentType  string `json:"content_type"`
	S3Url        string `json:"s3_url"`
	Sha256       string `json:"sha256"`
	Origin       string `json:"origin"`
	MediaId      string `json:"media_id"`
	CreatedTs    int64  `json:"created_ts"`
	Uploader     string `json:"uploader"`
}

type Manifest struct {
	Version   int                        `json:"version"`
	EntityId  string                     `json:"entity_id"`
	CreatedTs int64                      `json:"created_ts"`
	Media     map[string]*ManifestRecord `json:"media"`

	// Deprecated: for v1 manifests
	UserId string `json:"user_id,omitempty"`
}

func StartServerExport(serverName string, s3urls bool, includeData bool, ctx rcontext.RequestContext) (*types.BackgroundTask, string, error) {
	exportId, err := ids.NewUniqueId()
	if err != nil {
		return nil, "", err
	}

	db := storage.GetDatabase().GetMetadataStore(ctx)
	task, err := db.CreateBackgroundTask("export_data", map[string]interface{}{
		"server_name":     serverName,
		"include_s3_urls": s3urls,
		"include_data":    includeData,
		"export_id":       exportId,
	})

	if err != nil {
		return nil, "", err
	}

	go func() {
		// Use a new context in the goroutine
		ctx.Context = context.Background()
		db := storage.GetDatabase().GetMetadataStore(ctx)

		ds, err := datastore.PickDatastore(common.KindArchives, ctx)
		if err != nil {
			ctx.Log.Error(err)
			sentry.CaptureException(err)
			return
		}

		mediaDb := storage.GetDatabase().GetMediaStore(ctx)
		media, err := mediaDb.GetAllMediaForServer(serverName)
		if err != nil {
			ctx.Log.Error(err)
			sentry.CaptureException(err)
			return
		}

		compileArchive(exportId, serverName, ds, media, s3urls, includeData, ctx)

		ctx.Log.Info("Finishing export task")
		err = db.FinishedBackgroundTask(task.ID)
		if err != nil {
			ctx.Log.Error(err)
			ctx.Log.Error("Failed to flag task as finished")
			sentry.CaptureException(err)
		}
		ctx.Log.Info("Finished export")
	}()

	return task, exportId, nil
}

func StartUserExport(userId string, s3urls bool, includeData bool, ctx rcontext.RequestContext) (*types.BackgroundTask, string, error) {
	exportId, err := ids.NewUniqueId()
	if err != nil {
		return nil, "", err
	}

	db := storage.GetDatabase().GetMetadataStore(ctx)
	task, err := db.CreateBackgroundTask("export_data", map[string]interface{}{
		"user_id":         userId,
		"include_s3_urls": s3urls,
		"include_data":    includeData,
		"export_id":       exportId,
	})

	if err != nil {
		return nil, "", err
	}

	go func() {
		// Use a new context in the goroutine
		ctx.Context = context.Background()
		db := storage.GetDatabase().GetMetadataStore(ctx)

		ds, err := datastore.PickDatastore(common.KindArchives, ctx, )
		if err != nil {
			ctx.Log.Error(err)
			sentry.CaptureException(err)
			return
		}

		mediaDb := storage.GetDatabase().GetMediaStore(ctx)
		media, err := mediaDb.GetMediaByUser(userId)
		if err != nil {
			ctx.Log.Error(err)
			sentry.CaptureException(err)
			return
		}

		compileArchive(exportId, userId, ds, media, s3urls, includeData, ctx)

		ctx.Log.Info("Finishing export task")
		err = db.FinishedBackgroundTask(task.ID)
		if err != nil {
			ctx.Log.Error(err)
			ctx.Log.Error("Failed to flag task as finished")
			sentry.CaptureException(err)
		}
		ctx.Log.Info("Finished export")
	}()

	return task, exportId, nil
}

func compileArchive(exportId string, entityId string, archiveDs *datastore.DatastoreRef, media []*types.Media, s3urls bool, includeData bool, ctx rcontext.RequestContext) {
	exportDb := storage.GetDatabase().GetExportStore(ctx)
	err := exportDb.InsertExport(exportId, entityId)
	if err != nil {
		ctx.Log.Error(err)
		sentry.CaptureException(err)
		return
	}

	var currentTar *tar.Writer
	var currentTarBytes bytes.Buffer
	part := 0
	parts := make([]*types.ObjectInfo, 0)
	currentSize := int64(0)

	persistTar := func() error {
		_ = currentTar.Close()

		// compress
		ctx.Log.Info("Compressing tar file")
		gzipBytes := bytes.Buffer{}
		archiver := gzip.NewWriter(&gzipBytes)
		archiver.Name = fmt.Sprintf("export-part-%d.tar", part)
		_, err := io.Copy(archiver, util.BufferToStream(bytes.NewBuffer(currentTarBytes.Bytes())))
		if err != nil {
			return err
		}
		_ = archiver.Close()

		ctx.Log.Info("Uploading compressed tar file")
		buf := bytes.NewBuffer(gzipBytes.Bytes())
		size := int64(buf.Len())
		obj, err := archiveDs.UploadFile(util.BufferToStream(buf), size, ctx)
		if err != nil {
			return err
		}
		parts = append(parts, obj)

		fname := fmt.Sprintf("export-part-%d.tgz", part)
		err = exportDb.InsertExportPart(exportId, part, size, fname, archiveDs.DatastoreId, obj.Location)
		if err != nil {
			return err
		}

		return nil
	}

	newTar := func() error {
		if part > 0 {
			ctx.Log.Info("Persisting complete tar file")
			err := persistTar()
			if err != nil {
				return err
			}
		}

		ctx.Log.Info("Starting new tar file")
		currentTarBytes = bytes.Buffer{}
		currentTar = tar.NewWriter(&currentTarBytes)
		part = part + 1
		currentSize = 0

		return nil
	}

	// Start the first tar file
	ctx.Log.Info("Creating first tar file")
	err = newTar()
	if err != nil {
		ctx.Log.Error(err)
		sentry.CaptureException(err)
		return
	}

	putFile := func(name string, size int64, creationTime time.Time, file io.Reader) error {
		header := &tar.Header{
			Name:    name,
			Size:    size,
			Mode:    int64(0644),
			ModTime: creationTime,
		}
		err := currentTar.WriteHeader(header)
		if err != nil {
			ctx.Log.Error("error writing header")
			return err
		}

		i, err := io.Copy(currentTar, file)
		if err != nil {
			ctx.Log.Error("error writing file")
			return err
		}

		currentSize += i

		return nil
	}

	archivedName := func(m *types.Media) string {
		// TODO: Pick the right extension for the file type
		return fmt.Sprintf("%s__%s.obj", m.Origin, m.MediaId)
	}

	// Build a manifest first (JSON)
	ctx.Log.Info("Building manifest")
	indexModel := &templating.ExportIndexModel{
		Entity:   entityId,
		ExportID: exportId,
		Media:    make([]*templating.ExportIndexMediaModel, 0),
	}
	mediaManifest := make(map[string]*ManifestRecord)
	for _, m := range media {
		var s3url string
		if s3urls {
			s3url, err = ds_s3.GetS3URL(m.DatastoreId, m.Location)
			if err != nil {
				ctx.Log.Warn(err)
			}
		}
		mediaManifest[m.MxcUri()] = &ManifestRecord{
			ArchivedName: archivedName(m),
			FileName:     m.UploadName,
			SizeBytes:    m.SizeBytes,
			ContentType:  m.ContentType,
			S3Url:        s3url,
			Sha256:       m.Sha256Hash,
			Origin:       m.Origin,
			MediaId:      m.MediaId,
			CreatedTs:    m.CreationTs,
			Uploader:     m.UserId,
		}
		indexModel.Media = append(indexModel.Media, &templating.ExportIndexMediaModel{
			ExportID:        exportId,
			ArchivedName:    archivedName(m),
			FileName:        m.UploadName,
			SizeBytes:       m.SizeBytes,
			SizeBytesHuman:  humanize.Bytes(uint64(m.SizeBytes)),
			Origin:          m.Origin,
			MediaID:         m.MediaId,
			Sha256Hash:      m.Sha256Hash,
			ContentType:     m.ContentType,
			UploadTs:        m.CreationTs,
			UploadDateHuman: util.FromMillis(m.CreationTs).Format(time.UnixDate),
			Uploader:        m.UserId,
		})
	}
	manifest := &Manifest{
		Version:   2,
		EntityId:  entityId,
		CreatedTs: util.NowMillis(),
		Media:     mediaManifest,
	}
	b, err := json.Marshal(manifest)
	if err != nil {
		ctx.Log.Error(err)
		sentry.CaptureException(err)
		return
	}

	ctx.Log.Info("Writing manifest")
	err = putFile("manifest.json", int64(len(b)), time.Now(), util.BufferToStream(bytes.NewBuffer(b)))
	if err != nil {
		ctx.Log.Error(err)
		sentry.CaptureException(err)
		return
	}

	if includeData {
		ctx.Log.Info("Building and writing index")
		t, err := templating.GetTemplate("export_index")
		if err != nil {
			ctx.Log.Error(err)
			sentry.CaptureException(err)
			return
		}
		html := bytes.Buffer{}
		err = t.Execute(&html, indexModel)
		if err != nil {
			ctx.Log.Error(err)
			sentry.CaptureException(err)
			return
		}
		err = putFile("index.html", int64(html.Len()), time.Now(), util.BufferToStream(bytes.NewBuffer(html.Bytes())))
		if err != nil {
			ctx.Log.Error(err)
			sentry.CaptureException(err)
			return
		}

		ctx.Log.Info("Including data in the archive")
		for _, m := range media {
			ctx.Log.Info("Downloading ", m.MxcUri())
			s, err := datastore.DownloadStream(ctx, m.DatastoreId, m.Location)
			if err != nil {
				ctx.Log.Error(err)
				sentry.CaptureException(err)
				continue
			}

			ctx.Log.Infof("Copying %s to memory", m.MxcUri())
			b := bytes.Buffer{}
			_, err = io.Copy(&b, s)
			if err != nil {
				ctx.Log.Error(err)
				cleanup.DumpAndCloseStream(s)
				sentry.CaptureException(err)
				continue
			}
			cleanup.DumpAndCloseStream(s)
			s = util.BufferToStream(bytes.NewBuffer(b.Bytes()))

			ctx.Log.Info("Archiving ", m.MxcUri())
			err = putFile(archivedName(m), m.SizeBytes, time.Unix(0, m.CreationTs*int64(time.Millisecond)), s)
			if err != nil {
				ctx.Log.Error(err)
				sentry.CaptureException(err)
				return
			}

			if currentSize >= ctx.Config.Archiving.TargetBytesPerPart {
				ctx.Log.Info("Rotating tar")
				err = newTar()
				if err != nil {
					ctx.Log.Error(err)
					sentry.CaptureException(err)
					return
				}
			}
		}
	}

	if currentSize > 0 {
		ctx.Log.Info("Persisting last tar")
		err = persistTar()
		if err != nil {
			ctx.Log.Error(err)
			sentry.CaptureException(err)
			return
		}
	}
}
