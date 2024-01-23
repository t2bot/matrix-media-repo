package task_runner

import (
	"errors"
	"io"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/archival"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/datastores"
)

type ExportDataParams struct {
	UserId        string `json:"user_id,omitempty"`
	ServerName    string `json:"server_name,omitempty"`
	ExportId      string `json:"export_id"`
	IncludeS3Urls bool   `json:"include_s3_urls"`
}

func ExportData(ctx rcontext.RequestContext, task *database.DbTask) {
	defer markDone(ctx, task)

	params := ExportDataParams{}
	if err := task.Params.ApplyTo(&params); err != nil {
		markError(ctx, task, errors.Join(errors.New("error in decode"), err))
		ctx.Log.Error("Error decoding params: ", err)
		sentry.CaptureException(err)
		return
	}

	if params.ExportId == "" {
		markError(ctx, task, errors.New("missing export id"))
		ctx.Log.Error("No export ID provided")
		sentry.CaptureMessage("No export ID provided")
		return
	}

	exportDb := database.GetInstance().Exports.Prepare(ctx)
	if existingEntity, err := exportDb.GetEntity(params.ExportId); err != nil {
		markError(ctx, task, errors.Join(errors.New("error in validate"), err))
		ctx.Log.Error("Error checking export ID: ", err)
		sentry.CaptureException(err)
		return
	} else if existingEntity != "" {
		markError(ctx, task, errors.New("export id already in use"))
		ctx.Log.Error("Export ID already in use")
		sentry.CaptureMessage("Export ID already in use")
		return
	}

	entityId := params.UserId
	if entityId != "" && entityId[0] != '@' {
		markError(ctx, task, errors.New("invalid user id"))
		ctx.Log.Error("Invalid user ID")
		sentry.CaptureMessage("Invalid user ID")
		return
	} else if entityId == "" {
		entityId = params.ServerName
	}
	if entityId == "" {
		markError(ctx, task, errors.New("no entity provided"))
		ctx.Log.Error("No entity provided")
		sentry.CaptureMessage("No entity provided")
		return
	}

	if err := exportDb.Insert(params.ExportId, entityId); err != nil {
		markError(ctx, task, errors.Join(errors.New("error in persist"), err))
		ctx.Log.Error("Error persisting export ID: ", err)
		sentry.CaptureException(err)
		return
	}

	partsDb := database.GetInstance().ExportParts.Prepare(ctx)
	persistPart := func(partNum int, fileName string, data io.ReadCloser) error {
		dsConf, err := datastores.Pick(ctx, datastores.ArchivesKind)
		if err != nil {
			return err
		}
		sha256hash, sizeBytes, reader, err := datastores.BufferTemp(dsConf, data)
		if err != nil {
			return err
		}
		dsLocation, err := datastores.Upload(ctx, dsConf, reader, sizeBytes, "application/octet-stream", sha256hash)
		if err != nil {
			return err
		}
		if err = partsDb.Insert(&database.DbExportPart{
			ExportId:    params.ExportId,
			PartNum:     partNum,
			SizeBytes:   sizeBytes,
			FileName:    fileName,
			DatastoreId: dsConf.Id,
			Location:    dsLocation,
		}); err != nil {
			return err
		}
		return nil
	}

	if err := archival.ExportEntityData(ctx, params.ExportId, entityId, params.IncludeS3Urls, persistPart); err != nil {
		markError(ctx, task, errors.Join(errors.New("error in archival"), err))
		ctx.Log.Error("Error during export: ", err)
		sentry.CaptureException(err)
		return
	}
}
