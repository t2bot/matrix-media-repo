package data_controller

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/util/ids"
	"io"
	"net/http"
	"sync"

	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/upload_controller"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/storage/datastore/ds_s3"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type importUpdate struct {
	stop       bool
	fileMap    map[string]*bytes.Buffer
	onDoneChan chan bool
}

var openImports = &sync.Map{} // importId => updateChan

func VerifyImport(data io.Reader, ctx rcontext.RequestContext) (int, int, []string, error) {
	// Prepare the first update for the import (sync, so we can error)
	// We do this before anything else because if the archive is invalid then we shouldn't
	// even bother with an import.
	results, err := processArchive(data)
	if err != nil {
		return 0, 0, nil, err
	}

	manifestFile, ok := results["manifest.json"]
	if !ok {
		return 0, 0, nil, errors.New("no manifest provided in data package")
	}

	archiveManifest := &Manifest{}
	err = json.Unmarshal(manifestFile.Bytes(), archiveManifest)
	if err != nil {
		return 0, 0, nil, err
	}

	expected := 0
	found := 0
	missing := make([]string, 0)
	db := storage.GetDatabase().GetMediaStore(ctx)
	for mxc, r := range archiveManifest.Media {
		ctx.Log.Info("Checking file: ", mxc)
		expected++
		_, err = db.Get(r.Origin, r.MediaId)
		if err == nil {
			found++
		} else {
			missing = append(missing, mxc)
		}
	}

	return found, expected, missing, nil
}

func StartImport(data io.Reader, ctx rcontext.RequestContext) (*types.BackgroundTask, string, error) {
	// Prepare the first update for the import (sync, so we can error)
	// We do this before anything else because if the archive is invalid then we shouldn't
	// even bother with an import.
	results, err := processArchive(data)
	if err != nil {
		return nil, "", err
	}

	importId, err := ids.NewUniqueId()
	if err != nil {
		return nil, "", err
	}

	db := storage.GetDatabase().GetMetadataStore(ctx)
	task, err := db.CreateBackgroundTask("import_data", map[string]interface{}{
		"import_id": importId,
	})

	if err != nil {
		return nil, "", err
	}

	// Start the import and send it its first update
	updateChan := make(chan *importUpdate)
	go doImport(updateChan, task.ID, importId, ctx)
	openImports.Store(importId, updateChan)
	updateChan <- &importUpdate{stop: false, fileMap: results}

	return task, importId, nil
}

func IsImportWaiting(importId string) bool {
	runningImport, ok := openImports.Load(importId)
	if !ok || runningImport == nil {
		return false
	}
	return true
}

func AppendToImport(importId string, data io.Reader, withReturnChan bool) (chan bool, error) {
	runningImport, ok := openImports.Load(importId)
	if !ok || runningImport == nil {
		return nil, errors.New("import not found or it has been closed")
	}

	results, err := processArchive(data)
	if err != nil {
		return nil, err
	}

	// Repeat the safety check - the archive processing can take a bit
	runningImport, ok = openImports.Load(importId)
	if !ok || runningImport == nil {
		return nil, nil
	}

	var doneChan chan bool
	if withReturnChan {
		doneChan = make(chan bool)
	}
	updateChan := runningImport.(chan *importUpdate)
	updateChan <- &importUpdate{stop: false, fileMap: results, onDoneChan: doneChan}

	return doneChan, nil
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

func GetFileNames(data io.Reader) ([]string, error) {
	archiver, err := gzip.NewReader(data)
	if err != nil {
		return nil, err
	}

	defer archiver.Close()

	tarFile := tar.NewReader(archiver)
	names := make([]string, 0)
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

		names = append(names, header.Name)
	}

	return names, nil
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

func doImport(updateChannel chan *importUpdate, taskId int, importId string, ctx rcontext.RequestContext) {
	defer close(updateChannel)

	// Use a new context in the goroutine
	ctx.Context = context.Background()

	ctx.Log.Info("Preparing for import...")
	fileMap := make(map[string]*bytes.Buffer)
	stopImport := false
	archiveManifest := &Manifest{}
	haveManifest := false
	imported := make(map[string]bool)
	db := storage.GetDatabase().GetMediaStore(ctx)
	var update *importUpdate

	for !stopImport {
		if update != nil && update.onDoneChan != nil {
			ctx.Log.Info("Flagging tar as completed")
			update.onDoneChan <- true
		}
		update = <-updateChannel
		if update.stop {
			ctx.Log.Info("Close requested")
			stopImport = true
		}

		// Populate files
		for name, fileBytes := range update.fileMap {
			if _, ok := fileMap[name]; ok {
				ctx.Log.Warnf("Duplicate file name, skipping: %s", name)
				continue // file already known to us
			}
			ctx.Log.Infof("Tracking file: %s", name)
			fileMap[name] = fileBytes
		}

		var manifestBuf *bytes.Buffer
		var ok bool
		if manifestBuf, ok = fileMap["manifest.json"]; !ok {
			ctx.Log.Info("No manifest found - waiting for more files")
			continue
		}

		if !haveManifest {
			haveManifest = true
			err := json.Unmarshal(manifestBuf.Bytes(), archiveManifest)
			if err != nil {
				ctx.Log.Error("Failed to parse manifest - giving up on import")
				ctx.Log.Error(err)
				sentry.CaptureException(err)
				break
			}
			if archiveManifest.Version != 1 && archiveManifest.Version != 2 {
				ctx.Log.Error("Unsupported archive version")
				break
			}
			if archiveManifest.Version == 1 {
				archiveManifest.EntityId = archiveManifest.UserId
			}
			if archiveManifest.EntityId == "" {
				ctx.Log.Error("Invalid manifest: no entity")
				break
			}
			if archiveManifest.Media == nil {
				ctx.Log.Error("Invalid manifest: no media")
				break
			}
			ctx.Log.Infof("Using manifest for %s (v%d) created %d", archiveManifest.EntityId, archiveManifest.Version, archiveManifest.CreatedTs)
		}

		if !haveManifest {
			// Without a manifest we can't import anything
			continue
		}

		toClear := make([]string, 0)
		doClear := true
		for mxc, record := range archiveManifest.Media {
			_, found := imported[mxc]
			if found {
				continue // already imported
			}

			_, err := db.Get(record.Origin, record.MediaId)
			if err == nil {
				ctx.Log.Info("Media already imported: " + record.Origin + "/" + record.MediaId)

				// flag as imported and move on
				imported[mxc] = true
				continue
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
					ctx.Log.Errorf("Invalid user ID: %s", userId)
					serverName = ""
				} else {
					serverName = s
				}
			}
			if !util.IsServerOurs(serverName) {
				kind = common.KindRemoteMedia
			}

			ctx.Log.Infof("Attempting to import %s for %s", mxc, archiveManifest.EntityId)
			buf, found := fileMap[record.ArchivedName]
			if found {
				ctx.Log.Info("Using file from memory")
				closer := util.BufferToStream(buf)
				_, err := upload_controller.StoreDirect(nil, closer, record.SizeBytes, record.ContentType, record.FileName, userId, record.Origin, record.MediaId, kind, ctx, true)
				if err != nil {
					ctx.Log.Errorf("Error importing file: %s", err.Error())
					doClear = false // don't clear things on error
					sentry.CaptureException(err)
					continue
				}
				toClear = append(toClear, record.ArchivedName)
			} else if record.S3Url != "" {
				ctx.Log.Info("Using S3 URL")
				endpoint, bucket, location, err := ds_s3.ParseS3URL(record.S3Url)
				if err != nil {
					ctx.Log.Errorf("Error importing file: %s", err.Error())
					sentry.CaptureException(err)
					continue
				}

				ctx.Log.Infof("Seeing if a datastore for %s/%s exists", endpoint, bucket)
				datastores, err := datastore.GetAvailableDatastores(ctx)
				if err != nil {
					ctx.Log.Errorf("Error locating datastore: %s", err.Error())
					sentry.CaptureException(err)
					continue
				}
				imported := false
				for _, ds := range datastores {
					if ds.Type != "s3" {
						continue
					}

					tmplUrl, err := ds_s3.GetS3URL(ds.DatastoreId, location)
					if err != nil {
						ctx.Log.Errorf("Error investigating s3 datastore: %s", err.Error())
						sentry.CaptureException(err)
						continue
					}
					if tmplUrl == record.S3Url {
						ctx.Log.Infof("File matches! Assuming the file has been uploaded already")

						existingRecord, err := db.Get(record.Origin, record.MediaId)
						if err != nil && err != sql.ErrNoRows {
							ctx.Log.Errorf("Error testing file in database: %s", err.Error())
							sentry.CaptureException(err)
							break
						}
						if err != sql.ErrNoRows && existingRecord != nil {
							ctx.Log.Warnf("Media %s already exists - skipping without altering record", existingRecord.MxcUri())
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
							ctx.Log.Errorf("Error creating media record: %s", err.Error())
							sentry.CaptureException(err)
							break
						}

						ctx.Log.Infof("Media %s has been imported", media.MxcUri())
						imported = true
						break
					}
				}

				if !imported {
					ctx.Log.Info("No datastore found - trying to upload by downloading first")
					r, err := http.DefaultClient.Get(record.S3Url)
					if err != nil {
						ctx.Log.Errorf("Error trying to download file from S3 via HTTP: ", err.Error())
						sentry.CaptureException(err)
						continue
					}

					_, err = upload_controller.StoreDirect(nil, r.Body, r.ContentLength, record.ContentType, record.FileName, userId, record.Origin, record.MediaId, kind, ctx, true)
					if err != nil {
						ctx.Log.Errorf("Error importing file: %s", err.Error())
						sentry.CaptureException(err)
						continue
					}
				}
			} else {
				ctx.Log.Warn("Missing usable file for import - assuming it will show up in a future upload")
				continue
			}

			ctx.Log.Info("Counting file as imported")
			imported[mxc] = true
		}

		if doClear {
			ctx.Log.Info("Clearing up memory for imported files...")
			for _, f := range toClear {
				ctx.Log.Infof("Removing %s from memory", f)
				delete(fileMap, f)
			}
		}

		ctx.Log.Info("Checking for any unimported files...")
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
			ctx.Log.Info("No more files to import - closing import")
			stopImport = true
		}
	}

	// Clean up the last tar file
	if update != nil && update.onDoneChan != nil {
		ctx.Log.Info("Flagging tar as completed")
		update.onDoneChan <- true
	}

	openImports.Delete(importId)

	ctx.Log.Info("Finishing import task")
	dbMeta := storage.GetDatabase().GetMetadataStore(ctx)
	err := dbMeta.FinishedBackgroundTask(taskId)
	if err != nil {
		ctx.Log.Error(err)
		ctx.Log.Error("Failed to flag task as finished")
		sentry.CaptureException(err)
	}
	ctx.Log.Info("Finished import")
}
