package data_controller

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/controllers/upload_controller"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/storage/datastore/ds_s3"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type importUpdate struct {
	stop    bool
	fileMap map[string]*bytes.Buffer
}

var openImports = &sync.Map{} // importId => updateChan

func StartImport(data io.Reader, log *logrus.Entry) (*types.BackgroundTask, string, error) {
	ctx := context.Background()

	// Prepare the first update for the import (sync, so we can error)
	// We do this before anything else because if the archive is invalid then we shouldn't
	// even bother with an import.
	results, err := processArchive(data)
	if err != nil {
		return nil, "", err
	}

	importId, err := util.GenerateRandomString(128)
	if err != nil {
		return nil, "", err
	}

	db := storage.GetDatabase().GetMetadataStore(ctx, log)
	task, err := db.CreateBackgroundTask("import_data", map[string]interface{}{
		"import_id": importId,
	})

	if err != nil {
		return nil, "", err
	}

	// Start the import and send it its first update
	updateChan := make(chan *importUpdate)
	go doImport(updateChan, task.ID, importId, ctx, log)
	openImports.Store(importId, updateChan)
	updateChan <- &importUpdate{stop: false, fileMap: results}

	return task, importId, nil
}

func AppendToImport(importId string, data io.Reader) error {
	runningImport, ok := openImports.Load(importId)
	if !ok || runningImport == nil {
		return errors.New("import not found or it has been closed")
	}

	results, err := processArchive(data)
	if err != nil {
		return err
	}

	updateChan := runningImport.(chan *importUpdate)
	updateChan <- &importUpdate{stop: false, fileMap: results}

	return nil
}

func StopImport(importId string) error {
	runningImport, ok := openImports.Load(importId)
	if !ok || runningImport == nil {
		return errors.New("import not found or it has been closed")
	}

	updateChan := runningImport.(chan *importUpdate)
	updateChan <- &importUpdate{stop: true, fileMap: make(map[string]*bytes.Buffer)}

	return nil
}

func processArchive(data io.Reader) (map[string]*bytes.Buffer, error) {
	archiver, err := gzip.NewReader(data)
	if err != nil {
		return nil, err
	}

	defer archiver.Close()

	tarFile := tar.NewReader(archiver)
	index := make(map[string]*bytes.Buffer)
	for {
		header, err := tarFile.Next()
		if err == io.EOF {
			break // we're done
		}
		if err != nil {
			return nil, err
		}

		if header == nil {
			continue // skip this weird file
		}
		if header.Typeflag != tar.TypeReg {
			continue // skip directories and other stuff
		}

		// Copy the file into our index
		buf := &bytes.Buffer{}
		_, err = io.Copy(buf, tarFile)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewBuffer(buf.Bytes()) // clone to reset reader position
		index[header.Name] = buf
	}

	return index, nil
}

func doImport(updateChannel chan *importUpdate, taskId int, importId string, ctx context.Context, log *logrus.Entry) {
	log.Info("Preparing for import...")
	fileMap := make(map[string]*bytes.Buffer)
	stopImport := false
	archiveManifest := &manifest{}
	haveManifest := false
	imported := make(map[string]bool)
	db := storage.GetDatabase().GetMediaStore(ctx, log)

	for !stopImport {
		update := <-updateChannel
		if update.stop {
			log.Info("Close requested")
			stopImport = true
		}

		// Populate files
		for name, fileBytes := range update.fileMap {
			if _, ok := fileMap[name]; ok {
				log.Warnf("Duplicate file name, skipping: %s", name)
				continue // file already known to us
			}
			log.Infof("Tracking file: %s", name)
			fileMap[name] = fileBytes
		}

		// TODO: Search for a manifest and import a bunch of files
		var manifestBuf *bytes.Buffer
		var ok bool
		if manifestBuf, ok = fileMap["manifest.json"]; !ok {
			log.Info("No manifest found - waiting for more files")
			continue
		}

		if !haveManifest {
			haveManifest = true
			err := json.Unmarshal(manifestBuf.Bytes(), archiveManifest)
			if err != nil {
				log.Error("Failed to parse manifest - giving up on import")
				log.Error(err)
				break
			}
			if archiveManifest.Version != 1 && archiveManifest.Version != 2 {
				log.Error("Unsupported archive version")
				break
			}
			if archiveManifest.Version == 1 {
				archiveManifest.EntityId = archiveManifest.UserId
			}
			if archiveManifest.EntityId == "" {
				log.Error("Invalid manifest: no entity")
				break
			}
			if archiveManifest.Media == nil {
				log.Error("Invalid manifest: no media")
				break
			}
			log.Infof("Using manifest for %s (v%d) created %d", archiveManifest.EntityId, archiveManifest.Version, archiveManifest.CreatedTs)
		}

		if !haveManifest {
			// Without a manifest we can't import anything
			continue
		}

		for mxc, record := range archiveManifest.Media {
			_, found := imported[mxc]
			if found {
				continue // already imported
			}

			userId := archiveManifest.EntityId
			if userId[0] != '@' {
				userId = "" // assume none for now
			}

			kind := common.KindLocalMedia
			serverName := archiveManifest.EntityId
			if userId != "" {
				_, s, err := util.SplitUserId(userId)
				if err != nil {
					log.Errorf("Invalid user ID: %s", userId)
					serverName = ""
				} else {
					serverName = s
				}
			}
			if !util.IsServerOurs(serverName) {
				kind = common.KindRemoteMedia
			}

			log.Infof("Attempting to import %s for %s", mxc, archiveManifest.EntityId)
			buf, found := fileMap[record.ArchivedName]
			if found {
				log.Info("Using file from memory")
				closer := util.BufferToStream(buf)
				_, err := upload_controller.StoreDirect(closer, record.SizeBytes, record.ContentType, record.FileName, userId, record.Origin, record.MediaId, kind, ctx, log)
				if err != nil {
					log.Errorf("Error importing file: %s", err.Error())
					continue
				}
			} else if record.S3Url != "" {
				log.Info("Using S3 URL")
				endpoint, bucket, location, err := ds_s3.ParseS3URL(record.S3Url)
				if err != nil {
					log.Errorf("Error importing file: %s", err.Error())
					continue
				}

				log.Infof("Seeing if a datastore for %s/%s exists", endpoint, bucket)
				datastores, err := datastore.GetAvailableDatastores()
				if err != nil {
					log.Errorf("Error locating datastore: %s", err.Error())
					continue
				}
				imported := false
				for _, ds := range datastores {
					if ds.Type != "s3" {
						continue
					}

					tmplUrl, err := ds_s3.GetS3URL(ds.DatastoreId, location)
					if err != nil {
						log.Errorf("Error investigating s3 datastore: %s", err.Error())
						continue
					}
					if tmplUrl == record.S3Url {
						log.Infof("File matches! Assuming the file has been uploaded already")

						existingRecord, err := db.Get(record.Origin, record.MediaId)
						if err != nil && err != sql.ErrNoRows {
							log.Errorf("Error testing file in database: %s", err.Error())
							break
						}
						if err != sql.ErrNoRows && existingRecord != nil {
							log.Warnf("Media %s already exists - skipping without altering record", existingRecord.MxcUri())
							imported = true
							break
						}

						// Use the user ID (if any) as the uploader as a default. If this is an import
						// for a server then we use the recorded one, if any is available.
						uploader := userId
						if userId == "" {
							uploader = record.Uploader
						}
						media := &types.Media{
							Origin:      record.Origin,
							MediaId:     record.MediaId,
							UploadName:  record.FileName,
							ContentType: record.ContentType,
							UserId:      uploader,
							Sha256Hash:  record.Sha256,
							SizeBytes:   record.SizeBytes,
							DatastoreId: ds.DatastoreId,
							Location:    location,
							CreationTs:  record.CreatedTs,
						}

						err = db.Insert(media)
						if err != nil {
							log.Errorf("Error creating media record: %s", err.Error())
							break
						}

						log.Infof("Media %s has been imported", media.MxcUri())
						imported = true
						break
					}
				}

				if !imported {
					log.Info("No datastore found - trying to upload by downloading first")
					r, err := http.DefaultClient.Get(record.S3Url)
					if err != nil {
						log.Errorf("Error trying to download file from S3 via HTTP: ", err.Error())
						continue
					}

					_, err = upload_controller.StoreDirect(r.Body, r.ContentLength, record.ContentType, record.FileName, userId, record.Origin, record.MediaId, kind, ctx, log)
					if err != nil {
						log.Errorf("Error importing file: %s", err.Error())
						continue
					}
				}
			} else {
				log.Warn("Missing usable file for import - assuming it will show up in a future upload")
				continue
			}

			log.Info("Counting file as imported")
			imported[mxc] = true
		}

		missingAny := false
		for mxc, _ := range archiveManifest.Media {
			_, found := imported[mxc]
			if found {
				continue // already imported
			}
			missingAny = true
			break
		}

		if !missingAny {
			log.Info("No more files to import - closing import")
			stopImport = true
		}
	}

	openImports.Delete(importId)

	log.Info("Finishing import task")
	dbMeta := storage.GetDatabase().GetMetadataStore(ctx, log)
	err := dbMeta.FinishedBackgroundTask(taskId)
	if err != nil {
		log.Error(err)
		log.Error("Failed to flag task as finished")
	}
	log.Info("Finished import")
}
