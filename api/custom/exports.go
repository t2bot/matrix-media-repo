package custom

import (
	"bytes"
	"net/http"
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/api/_routers"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/datastores"
	"github.com/turt2live/matrix-media-repo/tasks"

	"github.com/dustin/go-humanize"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/matrix"
	"github.com/turt2live/matrix-media-repo/templating"
	"github.com/turt2live/matrix-media-repo/util"
)

type ExportStarted struct {
	ExportID string `json:"export_id"`
	TaskID   int    `json:"task_id"`
}

type ExportPartMetadata struct {
	Index     int    `json:"index"`
	SizeBytes int64  `json:"size"`
	FileName  string `json:"name"`
}

type ExportMetadata struct {
	Entity string                `json:"entity"`
	Parts  []*ExportPartMetadata `json:"parts"`
}

func ExportUserData(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	if !rctx.Config.Archiving.Enabled {
		return _responses.BadRequest("archiving is not enabled")
	}

	isAdmin := util.IsGlobalAdmin(user.UserId) || user.IsShared
	if !rctx.Config.Archiving.SelfService && !isAdmin {
		return _responses.AuthFailed()
	}

	s3urls := r.URL.Query().Get("s3_urls") != "false"

	userId := _routers.GetParam("userId", r)

	if !isAdmin && user.UserId != userId {
		return _responses.BadRequest("cannot export data for another user")
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"exportUserId": userId,
		"s3urls":       s3urls,
	})
	task, exportId, err := tasks.RunUserExport(rctx, userId, s3urls)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("fatal error starting export")
	}

	return &_responses.DoNotCacheResponse{Payload: &ExportStarted{
		TaskID:   task.TaskId,
		ExportID: exportId,
	}}
}

func ExportServerData(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	if !rctx.Config.Archiving.Enabled {
		return _responses.BadRequest("archiving is not enabled")
	}

	isAdmin := util.IsGlobalAdmin(user.UserId) || user.IsShared
	if !rctx.Config.Archiving.SelfService && !isAdmin {
		return _responses.AuthFailed()
	}

	s3urls := r.URL.Query().Get("s3_urls") != "false"

	serverName := _routers.GetParam("serverName", r)

	if !isAdmin {
		// They might be a local admin, so check that.

		// We won't be able to check unless we know about the homeserver though
		if !util.IsServerOurs(serverName) {
			return _responses.BadRequest("cannot export data for another server")
		}

		isLocalAdmin, err := matrix.IsUserAdmin(rctx, serverName, user.AccessToken, r.RemoteAddr)
		if err != nil {
			rctx.Log.Debug("Error verifying local admin: ", err)
			isLocalAdmin = false
		}
		if !isLocalAdmin {
			return _responses.BadRequest("cannot export data for another server")
		}
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"exportServerName": serverName,
		"s3urls":           s3urls,
	})
	task, exportId, err := tasks.RunServerExport(rctx, serverName, s3urls)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("fatal error starting export")
	}

	return &_responses.DoNotCacheResponse{Payload: &ExportStarted{
		TaskID:   task.TaskId,
		ExportID: exportId,
	}}
}

func ViewExport(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	if !rctx.Config.Archiving.Enabled {
		return _responses.BadRequest("archiving is not enabled")
	}

	exportId := _routers.GetParam("exportId", r)

	if !_routers.ServerNameRegex.MatchString(exportId) {
		_responses.BadRequest("invalid export ID")
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"exportId": exportId,
	})

	exportDb := database.GetInstance().Exports.Prepare(rctx)
	partsDb := database.GetInstance().ExportParts.Prepare(rctx)

	entityId, err := exportDb.GetEntity(exportId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to get entity for export ID")
	}
	if entityId == "" {
		return _responses.NotFoundError()
	}

	parts, err := partsDb.GetForExport(exportId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to get export parts")
	}

	template, err := templating.GetTemplate("view_export")
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to get template")
	}

	model := &templating.ViewExportModel{
		ExportID:    exportId,
		Entity:      entityId,
		ExportParts: make([]*templating.ViewExportPartModel, 0),
	}
	for _, p := range parts {
		model.ExportParts = append(model.ExportParts, &templating.ViewExportPartModel{
			ExportID:       exportId,
			Index:          p.PartNum,
			FileName:       p.FileName,
			SizeBytes:      p.SizeBytes,
			SizeBytesHuman: humanize.Bytes(uint64(p.SizeBytes)),
		})
	}

	html := bytes.Buffer{}
	err = template.Execute(&html, model)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to render template")
	}

	return &_responses.HtmlResponse{HTML: html.String()}
}

func GetExportMetadata(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	if !rctx.Config.Archiving.Enabled {
		return _responses.BadRequest("archiving is not enabled")
	}

	exportId := _routers.GetParam("exportId", r)

	if !_routers.ServerNameRegex.MatchString(exportId) {
		_responses.BadRequest("invalid export ID")
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"exportId": exportId,
	})

	exportDb := database.GetInstance().Exports.Prepare(rctx)
	partsDb := database.GetInstance().ExportParts.Prepare(rctx)

	entityId, err := exportDb.GetEntity(exportId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to get entity for export ID")
	}

	parts, err := partsDb.GetForExport(exportId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to get export parts")
	}

	metadata := &ExportMetadata{
		Entity: entityId,
		Parts:  make([]*ExportPartMetadata, 0),
	}
	for _, p := range parts {
		metadata.Parts = append(metadata.Parts, &ExportPartMetadata{
			Index:     p.PartNum,
			SizeBytes: p.SizeBytes,
			FileName:  p.FileName,
		})
	}

	return &_responses.DoNotCacheResponse{Payload: metadata}
}

func DownloadExportPart(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	if !rctx.Config.Archiving.Enabled {
		return _responses.BadRequest("archiving is not enabled")
	}

	exportId := _routers.GetParam("exportId", r)
	pid := _routers.GetParam("partId", r)

	if !_routers.ServerNameRegex.MatchString(exportId) {
		_responses.BadRequest("invalid export ID")
	}

	partId, err := strconv.Atoi(pid)
	if err != nil {
		rctx.Log.Error(err)
		return _responses.BadRequest("invalid part index")
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"exportId": exportId,
		"partId":   partId,
	})

	partsDb := database.GetInstance().ExportParts.Prepare(rctx)
	part, err := partsDb.Get(exportId, partId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to get part")
	}

	if part == nil {
		return _responses.NotFoundError()
	}

	dsConf, ok := datastores.Get(rctx, part.DatastoreId)
	if !ok {
		sentry.CaptureMessage("failed to locate datastore")
		return _responses.InternalServerError("failed to locate datastore")
	}
	s, err := datastores.Download(rctx, dsConf, part.Location)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to start download")
	}

	return &_responses.DownloadResponse{
		ContentType:       "application/gzip", // TODO: We should be detecting type rather than assuming
		SizeBytes:         part.SizeBytes,
		Data:              s,
		Filename:          part.FileName,
		TargetDisposition: "attachment",
	}
}

func DeleteExport(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	if !rctx.Config.Archiving.Enabled {
		return _responses.BadRequest("archiving is not enabled")
	}

	exportId := _routers.GetParam("exportId", r)

	if !_routers.ServerNameRegex.MatchString(exportId) {
		_responses.BadRequest("invalid export ID")
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"exportId": exportId,
	})

	exportDb := database.GetInstance().Exports.Prepare(rctx)
	partsDb := database.GetInstance().ExportParts.Prepare(rctx)

	rctx.Log.Info("Getting information on which parts to delete")
	parts, err := partsDb.GetForExport(exportId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to get export parts")
	}

	for _, part := range parts {
		rctx.Log.Debugf("Deleting object '%s' from datastore '%s'", part.Location, part.DatastoreId)
		err = datastores.RemoveWithDsId(rctx, part.DatastoreId, part.Location)
		if err != nil {
			rctx.Log.Error(err)
			sentry.CaptureException(err)
			return _responses.InternalServerError("failed to delete export part")
		}
	}

	rctx.Log.Debug("Purging export from database")
	err = partsDb.DeleteForExport(exportId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to delete export parts")
	}
	err = exportDb.Delete(exportId)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("failed to delete export record")
	}

	return _responses.EmptyResponse{}
}
