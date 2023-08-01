package task_runner

import (
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/datastores"
)

type DatastoreMigrateParams struct {
	SourceDsId string `json:"source_datastore_id"`
	TargetDsId string `json:"target_datastore_id"`
	BeforeTs   int64  `json:"before_ts"`
}

func DatastoreMigrate(ctx rcontext.RequestContext, task *database.DbTask) {
	defer markDone(ctx, task)

	params := DatastoreMigrateParams{}
	if err := task.Params.ApplyTo(&params); err != nil {
		ctx.Log.Error("Error decoding params: ", err)
		sentry.CaptureException(err)
		return
	}

	if params.SourceDsId == params.TargetDsId {
		ctx.Log.Error("Source and target datastore are the same")
		return
	}

	sourceDs, ok := datastores.Get(ctx, params.SourceDsId)
	if !ok {
		ctx.Log.Error("Unable to locate source datastore ID")
		return
	}

	targetDs, ok := datastores.Get(ctx, params.TargetDsId)
	if !ok {
		ctx.Log.Error("Unable to locate target datastore ID")
		return
	}

	db := database.GetInstance().MetadataView.Prepare(ctx)

	if records, err := db.GetMediaForDatastoreByLastAccess(params.SourceDsId, params.BeforeTs); err != nil {
		ctx.Log.Error("Error getting movable media: ", err)
		sentry.CaptureException(err)
		return
	} else {
		moveDatastoreObjects(ctx, records, sourceDs, targetDs)
	}

	if records, err := db.GetThumbnailsForDatastoreByLastAccess(params.SourceDsId, params.BeforeTs); err != nil {
		ctx.Log.Error("Error getting movable thumbnails: ", err)
		sentry.CaptureException(err)
		return
	} else {
		moveDatastoreObjects(ctx, records, sourceDs, targetDs)
	}
}

func moveDatastoreObjects(ctx rcontext.RequestContext, records []*database.VirtLastAccess, sourceDs config.DatastoreConfig, targetDs config.DatastoreConfig) {
	mediaDb := database.GetInstance().Media.Prepare(ctx)
	thumbsDb := database.GetInstance().Thumbnails.Prepare(ctx)
	done := make(map[string]bool)
	for _, record := range records {
		doneId := fmt.Sprintf("%s/%s", record.DatastoreId, record.Location)
		if _, ok := done[doneId]; ok {
			continue
		}

		recordCtx := ctx.LogWithFields(logrus.Fields{"sha256": record.Sha256Hash, "dsId": record.DatastoreId, "location": record.Location})
		recordCtx.Log.Debug("Moving record")

		sourceStream, err := datastores.Download(recordCtx, sourceDs, record.Location)
		if err != nil {
			recordCtx.Log.Error("Failed to start download from source: ", err)
			sentry.CaptureException(err)
			continue
		}

		newLocation, err := datastores.Upload(recordCtx, targetDs, sourceStream, record.SizeBytes, record.ContentType, record.Sha256Hash)
		if err != nil {
			recordCtx.Log.Error("Failed to upload to target: ", err)
			sentry.CaptureException(err)
			continue
		}

		if err = mediaDb.UpdateLocation(record.DatastoreId, record.Location, targetDs.Id, newLocation); err != nil {
			recordCtx.Log.Error("Failed to update media table with new datastore and location: ", err)
			sentry.CaptureException(err)
			continue
		}

		if err = thumbsDb.UpdateLocation(record.DatastoreId, record.Location, targetDs.Id, newLocation); err != nil {
			recordCtx.Log.Error("Failed to update thumbnails table with new datastore and location: ", err)
			sentry.CaptureException(err)
			continue
		}

		if err = datastores.Remove(recordCtx, sourceDs, record.Location); err != nil {
			recordCtx.Log.Error("Failed to remove source object from datastore: ", err)
			sentry.CaptureException(err)
			continue
		}

		done[doneId] = true
	}
}
