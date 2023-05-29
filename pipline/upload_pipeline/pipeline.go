package upload_pipeline

import (
	"errors"
	"io"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/datastores"
	"github.com/turt2live/matrix-media-repo/pipline/_steps/quota"
	"github.com/turt2live/matrix-media-repo/pipline/_steps/upload"
	"github.com/turt2live/matrix-media-repo/util"
)

// UploadMedia Media upload. If mediaId is an empty string, one will be generated.
func UploadMedia(ctx rcontext.RequestContext, origin string, mediaId string, r io.ReadCloser, contentType string, fileName string, userId string, kind datastores.Kind) (*database.DbMedia, error) {
	// Step 1: Limit the stream's length
	r = upload.LimitStream(ctx, r)

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

	// Step 4: Buffer to the datastore's temporary path
	sha256hash, sizeBytes, reader, err := datastores.BufferTemp(dsConf, r)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	// Step 5: Split the buffer to calculate a blurhash & populate cache later
	bhR, bhW := io.Pipe()
	cacheR, cacheW := io.Pipe()
	allWriters := io.MultiWriter(cacheW, bhW)
	tee := io.TeeReader(reader, allWriters)

	defer bhW.CloseWithError(errors.New("failed to finish write"))
	defer cacheW.CloseWithError(errors.New("failed to finish write"))

	// Step 6: Check quarantine
	if err = upload.CheckQuarantineStatus(ctx, sha256hash); err != nil {
		return nil, err
	}

	// Step 7: Ensure user can upload within quota
	err = quota.CanUpload(ctx, userId, sizeBytes)
	if err != nil {
		return nil, err
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
		Sha256Hash:  sha256hash,
		SizeBytes:   sizeBytes,
		CreationTs:  util.NowMillis(),
		Quarantined: false,
		DatastoreId: "", // Populated later
		Location:    "", // Populated later
	}
	record, perfect, err := upload.FindRecord(ctx, sha256hash, userId, contentType, fileName)
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
			return newRecord, nil
		}
	}

	// Step 10: Asynchronously calculate blurhash
	bhChan := upload.CalculateBlurhashAsync(ctx, bhR, sha256hash)

	// Step 11: Asynchronously upload to cache
	cacheChan := upload.PopulateCacheAsync(ctx, cacheR, sizeBytes, sha256hash)

	// Step 12: Since we didn't find a duplicate, upload it to the datastore
	dsLocation, err := datastores.Upload(ctx, dsConf, io.NopCloser(tee), sizeBytes, contentType, sha256hash)
	if err != nil {
		return nil, err
	}
	if err = bhW.Close(); err != nil {
		ctx.Log.Warn("Failed to close writer for blurhash: ", err)
		close(bhChan)
	}
	if err = cacheW.Close(); err != nil {
		ctx.Log.Warn("Failed to close writer for cache layer: ", err)
		close(cacheChan)
	}

	// Step 13: Wait for channels
	<-bhChan
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
	return newRecord, nil
}
