package pipeline_upload

import (
	"errors"
	"io"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/datastores"
	"github.com/t2bot/matrix-media-repo/notifier"
	"github.com/t2bot/matrix-media-repo/pipelines/_steps/meta"
	"github.com/t2bot/matrix-media-repo/pipelines/_steps/quota"
	"github.com/t2bot/matrix-media-repo/pipelines/_steps/upload"
	"github.com/t2bot/matrix-media-repo/restrictions"
	"github.com/t2bot/matrix-media-repo/util"
	"github.com/t2bot/matrix-media-repo/util/readers"
)

// Execute Media upload. If mediaId is an empty string, one will be generated.
func Execute(ctx rcontext.RequestContext, origin string, mediaId string, r io.ReadCloser, contentType string, fileName string, userId string, kind datastores.Kind) (*database.DbMedia, error) {
	uploadDone := func(record *database.DbMedia) {
		meta.FlagAccess(ctx, record.Sha256Hash, 0) // upload time is zero here to skip metrics gathering
		if err := notifier.UploadDone(ctx, record); err != nil {
			ctx.Log.Warn("Non-fatal error notifying about completed upload: ", err)
			sentry.CaptureException(err)
		}
	}

	// Step 1: Limit the stream's length
	if kind == datastores.LocalMediaKind {
		r = upload.LimitStream(ctx, r)
	}

	// Step 2: Create a media ID (if needed)
	mustUseMediaId := true
	if mediaId == "" {
		var err error
		mediaId, err = upload.GenerateMediaId(ctx, origin)
		if err != nil {
			return nil, err
		}
		mustUseMediaId = false
	}

	// Step 3: Pick a datastore
	dsConf, err := datastores.Pick(ctx, kind)
	if err != nil {
		return nil, err
	}

	// Step 4: Buffer to the datastore's temporary path, and check for spam
	spamR, spamW := io.Pipe()
	spamTee := io.TeeReader(r, spamW)
	spamChan := upload.CheckSpamAsync(ctx, spamR, upload.FileMetadata{
		Name:        fileName,
		ContentType: contentType,
		UserId:      userId,
		Origin:      origin,
		MediaId:     mediaId,
	})
	sha256hash, sizeBytes, reader, err := datastores.BufferTemp(dsConf, readers.NewCancelCloser(io.NopCloser(spamTee), func() {
		r.Close()
	}))
	if err != nil {
		return nil, err
	}
	if err = spamW.Close(); err != nil {
		ctx.Log.Warn("Failed to close writer for spam checker: ", err)
		spamChan <- upload.SpamResponse{Err: errors.New("failed to close")}
	}
	defer reader.Close()
	spam := <-spamChan
	if spam.Err != nil {
		return nil, err
	}
	if spam.IsSpam {
		return nil, common.ErrMediaQuarantined
	}

	// Step 5: Split the buffer to populate cache later
	cacheR, cacheW := io.Pipe()
	allWriters := io.MultiWriter(cacheW)
	tee := io.TeeReader(reader, allWriters)

	defer func(cacheW *io.PipeWriter, err error) {
		_ = cacheW.CloseWithError(err)
	}(cacheW, errors.New("failed to finish write"))

	// Step 6: Check quarantine
	if err = upload.CheckQuarantineStatus(ctx, sha256hash); err != nil {
		return nil, err
	}

	// Step 7: Ensure user can upload within quota
	if userId != "" && !config.Runtime.IsImportProcess {
		err = quota.CanUpload(ctx, userId, sizeBytes)
		if err != nil {
			return nil, err
		}
	}

	// Step 8: Acquire a lock on the media hash for uploading
	unlockFn, err := upload.LockForUpload(ctx, sha256hash)
	if err != nil {
		return nil, err
	}
	//goland:noinspection GoUnhandledErrorResult
	defer unlockFn()

	// Step 9: Pull all upload records (to check if an upload has already happened)
	newRecord := &database.DbMedia{
		Origin:      origin,
		MediaId:     mediaId,
		UploadName:  fileName,
		ContentType: contentType,
		UserId:      userId,
		SizeBytes:   sizeBytes,
		CreationTs:  util.NowMillis(),
		Quarantined: false,
		Locatable: &database.Locatable{
			Sha256Hash:  sha256hash,
			DatastoreId: "", // Populated later
			Location:    "", // Populated later
		},
	}
	record, perfect, err := upload.FindRecord(ctx, sha256hash, userId, contentType, fileName)
	if err != nil {
		return nil, err
	}
	if record != nil {
		// We already had this record in some capacity
		if perfect && !mustUseMediaId {
			// Exact match - deduplicate, skip upload to datastore
			return record, nil
		} else {
			// We already uploaded it somewhere else - use the datastore ID and location
			newRecord.Quarantined = record.Quarantined // just in case (shouldn't be a different value by here)
			newRecord.DatastoreId = record.DatastoreId
			newRecord.Location = record.Location
			if err = database.GetInstance().Media.Prepare(ctx).Insert(newRecord); err != nil {
				return nil, err
			}
			if config.Get().General.FreezeUnauthenticatedMedia {
				if err = restrictions.SetMediaRequiresAuth(ctx, newRecord.Origin, newRecord.MediaId); err != nil {
					return nil, err
				}
			}
			uploadDone(newRecord)
			return newRecord, nil
		}
	}

	// Step 11: Asynchronously upload to cache
	cacheChan := upload.PopulateCacheAsync(ctx, cacheR, sizeBytes, sha256hash)

	// Step 12: Since we didn't find a duplicate, upload it to the datastore
	dsLocation, err := datastores.Upload(ctx, dsConf, io.NopCloser(tee), sizeBytes, contentType, sha256hash)
	if err != nil {
		return nil, err
	}
	if err = cacheW.Close(); err != nil {
		ctx.Log.Warn("Failed to close writer for cache layer: ", err)
		close(cacheChan)
	}

	// Step 13: Wait for channels
	<-cacheChan

	// Step 14: Everything finally looks good - return some stuff
	newRecord.DatastoreId = dsConf.Id
	newRecord.Location = dsLocation
	if err = database.GetInstance().Media.Prepare(ctx).Insert(newRecord); err != nil {
		if err2 := datastores.Remove(ctx, dsConf, dsLocation); err2 != nil {
			sentry.CaptureException(err2)
			ctx.Log.Warn("Error deleting upload (delete attempted due to persistence error): ", err2)
		}
		return nil, err
	}
	if config.Get().General.FreezeUnauthenticatedMedia {
		if err = restrictions.SetMediaRequiresAuth(ctx, newRecord.Origin, newRecord.MediaId); err != nil {
			return nil, err
		}
	}
	uploadDone(newRecord)
	return newRecord, nil
}
