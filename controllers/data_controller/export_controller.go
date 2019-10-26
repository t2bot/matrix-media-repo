package data_controller

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type manifestRecord struct {
	FileName     string `json:"name"`
	ArchivedName string `json:"file_name"`
	SizeBytes    int64  `json:"size_bytes"`
	ContentType  string `json:"content_type"`
	S3Url        string `json:"s3_url"`
	Sha256       string `json:"sha256"`
	Origin       string `json:"origin"`
	MediaId      string `json:"media_id"`
	CreatedTs    int64  `json:"created_ts"`
}

type manifest struct {
	Version   int                        `json:"version"`
	UserId    string                     `json:"user_id"`
	CreatedTs int64                      `json:"created_ts"`
	Media     map[string]*manifestRecord `json:"media"`
}

func StartUserExport(userId string, s3urls bool, includeData bool, log *logrus.Entry) (*types.BackgroundTask, string, error) {
	ctx := context.Background()

	exportId, err := util.GenerateRandomString(128)
	if err != nil {
		return nil, "", err
	}

	db := storage.GetDatabase().GetMetadataStore(ctx, log)
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
		ds, err := datastore.PickDatastore(common.KindArchives, ctx, log)
		if err != nil {
			log.Error(err)
			return
		}

		mediaDb := storage.GetDatabase().GetMediaStore(ctx, log)
		media, err := mediaDb.GetMediaByUser(userId)
		if err != nil {
			log.Error(err)
			return
		}

		exportDb := storage.GetDatabase().GetExportStore(ctx, log)
		err = exportDb.InsertExport(exportId, userId)
		if err != nil {
			log.Error(err)
			return
		}

		var currentTar *tar.Writer
		var currentTarBytes bytes.Buffer
		part := 0
		parts := make([]*types.ObjectInfo, 0)
		currentSize := int64(0)

		persistTar := func() error {
			currentTar.Close()

			// compress
			log.Info("Compressing tar file")
			gzipBytes := bytes.Buffer{}
			archiver := gzip.NewWriter(&gzipBytes)
			archiver.Name = fmt.Sprintf("export-part-%d.tar", part)
			_, err := io.Copy(archiver, util.BufferToStream(bytes.NewBuffer(currentTarBytes.Bytes())))
			if err != nil {
				return err
			}
			archiver.Close()

			log.Info("Uploading compressed tar file")
			buf := bytes.NewBuffer(gzipBytes.Bytes())
			size := int64(buf.Len())
			obj, err := ds.UploadFile(util.BufferToStream(buf), size, ctx, log)
			if err != nil {
				return err
			}
			parts = append(parts, obj)

			fname := fmt.Sprintf("export-part-%d.tgz", part)
			err = exportDb.InsertExportPart(exportId, part, size, fname, ds.DatastoreId, obj.Location)
			if err != nil {
				return err
			}

			return nil
		}

		newTar := func() error {
			if part > 0 {
				log.Info("Persisting complete tar file")
				err := persistTar()
				if err != nil {
					return err
				}
			}

			log.Info("Starting new tar file")
			currentTarBytes = bytes.Buffer{}
			currentTar = tar.NewWriter(&currentTarBytes)
			part = part + 1
			currentSize = 0

			return nil
		}

		// Start the first tar file
		log.Info("Creating first tar file")
		err = newTar()
		if err != nil {
			log.Error(err)
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
				log.Error("error writing header")
				return err
			}

			i, err := io.Copy(currentTar, file)
			if err != nil {
				log.Error("error writing file")
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
		log.Info("Building manifest")
		mediaManifest := make(map[string]*manifestRecord)
		for _, m := range media {
			mediaManifest[m.MxcUri()] = &manifestRecord{
				ArchivedName: archivedName(m),
				FileName:     m.UploadName,
				SizeBytes:    m.SizeBytes,
				ContentType:  m.ContentType,
				S3Url:        "TODO",
				Sha256:       m.Sha256Hash,
				Origin:       m.Origin,
				MediaId:      m.MediaId,
				CreatedTs:    m.CreationTs,
			}
		}
		manifest := &manifest{
			Version:   1,
			UserId:    userId,
			CreatedTs: util.NowMillis(),
			Media:     mediaManifest,
		}
		b, err := json.Marshal(manifest)
		if err != nil {
			log.Error(err)
			return
		}

		log.Info("Writing manifest")
		err = putFile("manifest.json", int64(len(b)), time.Now(), util.BufferToStream(bytes.NewBuffer(b)))
		if err != nil {
			log.Error(err)
			return
		}

		if includeData {
			log.Info("Including data in the archive")
			for _, m := range media {
				log.Info("Downloading ", m.MxcUri())
				s, err := datastore.DownloadStream(ctx, log, m.DatastoreId, m.Location)
				if err != nil {
					log.Error(err)
					continue
				}

				log.Infof("Copying %s to memory", m.MxcUri())
				b := bytes.Buffer{}
				_, err = io.Copy(&b, s)
				if err != nil {
					log.Error(err)
					continue
				}
				s.Close()
				s = util.BufferToStream(bytes.NewBuffer(b.Bytes()))

				log.Info("Archiving ", m.MxcUri())
				err = putFile(archivedName(m), m.SizeBytes, time.Unix(0, m.CreationTs*int64(time.Millisecond)), s)
				if err != nil {
					log.Error(err)
					return
				}

				if currentSize >= config.Get().Archiving.TargetBytesPerPart {
					log.Info("Rotating tar")
					err = newTar()
					if err != nil {
						log.Error(err)
						return
					}
				}
			}
		}

		if currentSize > 0 {
			log.Info("Persisting last tar")
			err = persistTar()
			if err != nil {
				log.Error(err)
				return
			}
		}

		log.Info("Finishing export task")
		err = db.FinishedBackgroundTask(task.ID)
		if err != nil {
			log.Error(err)
			log.Error("Failed to flag task as finished")
		}
		log.Info("Finished export")
	}()

	return task, exportId, nil
}
