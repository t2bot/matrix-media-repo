package pipeline_upload

import (
	"io"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/datastores"
	"github.com/turt2live/matrix-media-repo/util"
)

func ExecutePut(ctx rcontext.RequestContext, origin string, mediaId string, r io.ReadCloser, contentType string, fileName string, userId string) (*database.DbMedia, error) {
	// Step 1: Do we already have a media record for this?
	mediaDb := database.GetInstance().Media.Prepare(ctx)
	mediaRecord, err := mediaDb.GetById(origin, mediaId)
	if err != nil {
		return nil, err
	}
	if mediaRecord != nil {
		return nil, common.ErrAlreadyUploaded
	}

	// Step 2: Try to find the holding record
	expiringDb := database.GetInstance().ExpiringMedia.Prepare(ctx)
	record, err := expiringDb.Get(origin, mediaId)
	if err != nil {
		return nil, err
	}

	// Step 3: Is the record expired?
	if record == nil || record.ExpiresTs < util.NowMillis() {
		return nil, common.ErrExpired
	}

	// Step 4: Is the correct user uploading this media?
	if record.UserId != userId {
		return nil, common.ErrWrongUser
	}

	// Step 5: Do the upload
	newRecord, err := Execute(ctx, origin, mediaId, r, contentType, fileName, userId, datastores.LocalMediaKind)
	if err != nil {
		return nil, err
	}

	// Step 6: Delete the holding record
	if err2 := expiringDb.Delete(origin, mediaId); err2 != nil {
		ctx.Log.Warn("Non-fatal error while deleting expiring media record: " + err2.Error())
		sentry.CaptureException(err2)
	}

	return newRecord, err
}
