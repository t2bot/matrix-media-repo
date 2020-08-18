package custom

import (
	"bytes"
	"net/http"
	"strconv"

	"github.com/dustin/go-humanize"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/api/r0"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/data_controller"
	"github.com/turt2live/matrix-media-repo/matrix"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
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

func ExportUserData(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	if !rctx.Config.Archiving.Enabled {
		return api.BadRequest("archiving is not enabled")
	}

	isAdmin := util.IsGlobalAdmin(user.UserId) || user.IsShared
	if !rctx.Config.Archiving.SelfService && !isAdmin {
		return api.AuthFailed()
	}

	includeData := r.URL.Query().Get("include_data") != "false"
	s3urls := r.URL.Query().Get("s3_urls") != "false"

	params := mux.Vars(r)

	userId := params["userId"]

	if !isAdmin && user.UserId != userId {
		return api.BadRequest("cannot export data for another user")
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"exportUserId": userId,
		"includeData":  includeData,
		"s3urls":       s3urls,
	})
	task, exportId, err := data_controller.StartUserExport(userId, s3urls, includeData, rctx)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("fatal error starting export")
	}

	return &api.DoNotCacheResponse{Payload: &ExportStarted{
		TaskID:   task.ID,
		ExportID: exportId,
	}}
}

func ExportServerData(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	if !rctx.Config.Archiving.Enabled {
		return api.BadRequest("archiving is not enabled")
	}

	isAdmin := util.IsGlobalAdmin(user.UserId) || user.IsShared
	if !rctx.Config.Archiving.SelfService && !isAdmin {
		return api.AuthFailed()
	}

	includeData := r.URL.Query().Get("include_data") != "false"
	s3urls := r.URL.Query().Get("s3_urls") != "false"

	params := mux.Vars(r)

	serverName := params["serverName"]

	if !isAdmin {
		// They might be a local admin, so check that.

		// We won't be able to check unless we know about the homeserver though
		if !util.IsServerOurs(serverName) {
			return api.BadRequest("cannot export data for another server")
		}

		isLocalAdmin, err := matrix.IsUserAdmin(rctx, serverName, user.AccessToken, r.RemoteAddr)
		if err != nil {
			rctx.Log.Error("Error verifying local admin: " + err.Error())
			isLocalAdmin = false
		}
		if !isLocalAdmin {
			return api.BadRequest("cannot export data for another server")
		}
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"exportServerName": serverName,
		"includeData":      includeData,
		"s3urls":           s3urls,
	})
	task, exportId, err := data_controller.StartServerExport(serverName, s3urls, includeData, rctx)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("fatal error starting export")
	}

	return &api.DoNotCacheResponse{Payload: &ExportStarted{
		TaskID:   task.ID,
		ExportID: exportId,
	}}
}

func ViewExport(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	if !rctx.Config.Archiving.Enabled {
		return api.BadRequest("archiving is not enabled")
	}

	params := mux.Vars(r)

	exportId := params["exportId"]
	rctx = rctx.LogWithFields(logrus.Fields{
		"exportId": exportId,
	})

	exportDb := storage.GetDatabase().GetExportStore(rctx)

	exportInfo, err := exportDb.GetExportMetadata(exportId)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("failed to get metadata")
	}

	parts, err := exportDb.GetExportParts(exportId)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("failed to get export parts")
	}

	template, err := templating.GetTemplate("view_export")
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("failed to get template")
	}

	model := &templating.ViewExportModel{
		ExportID:    exportInfo.ExportID,
		Entity:      exportInfo.Entity,
		ExportParts: make([]*templating.ViewExportPartModel, 0),
	}
	for _, p := range parts {
		model.ExportParts = append(model.ExportParts, &templating.ViewExportPartModel{
			ExportID:       exportInfo.ExportID,
			Index:          p.Index,
			FileName:       p.FileName,
			SizeBytes:      p.SizeBytes,
			SizeBytesHuman: humanize.Bytes(uint64(p.SizeBytes)),
		})
	}

	html := bytes.Buffer{}
	err = template.Execute(&html, model)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("failed to render template")
	}

	return &api.HtmlResponse{HTML: string(html.Bytes())}
}

func GetExportMetadata(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	if !rctx.Config.Archiving.Enabled {
		return api.BadRequest("archiving is not enabled")
	}

	params := mux.Vars(r)

	exportId := params["exportId"]
	rctx = rctx.LogWithFields(logrus.Fields{
		"exportId": exportId,
	})

	exportDb := storage.GetDatabase().GetExportStore(rctx)

	exportInfo, err := exportDb.GetExportMetadata(exportId)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("failed to get metadata")
	}

	parts, err := exportDb.GetExportParts(exportId)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("failed to get export parts")
	}

	metadata := &ExportMetadata{
		Entity: exportInfo.Entity,
		Parts:  make([]*ExportPartMetadata, 0),
	}
	for _, p := range parts {
		metadata.Parts = append(metadata.Parts, &ExportPartMetadata{
			Index:     p.Index,
			SizeBytes: p.SizeBytes,
			FileName:  p.FileName,
		})
	}

	return &api.DoNotCacheResponse{Payload: metadata}
}

func DownloadExportPart(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	if !rctx.Config.Archiving.Enabled {
		return api.BadRequest("archiving is not enabled")
	}

	params := mux.Vars(r)

	exportId := params["exportId"]
	partId, err := strconv.ParseInt(params["partId"], 10, 64)
	if err != nil {
		rctx.Log.Error(err)
		return api.BadRequest("invalid part index")
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"exportId": exportId,
		"partId":   partId,
	})

	db := storage.GetDatabase().GetExportStore(rctx)
	part, err := db.GetExportPart(exportId, int(partId))
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("failed to get part")
	}

	s, err := datastore.DownloadStream(rctx, part.DatastoreID, part.Location)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("failed to start download")
	}

	return &r0.DownloadMediaResponse{
		ContentType: "application/gzip",
		SizeBytes:   part.SizeBytes,
		Data:        s,
		Filename:    part.FileName,
		TargetDisposition: "attachment",
	}
}

func DeleteExport(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	if !rctx.Config.Archiving.Enabled {
		return api.BadRequest("archiving is not enabled")
	}

	params := mux.Vars(r)

	exportId := params["exportId"]

	rctx = rctx.LogWithFields(logrus.Fields{
		"exportId": exportId,
	})

	db := storage.GetDatabase().GetExportStore(rctx)

	rctx.Log.Info("Getting information on which parts to delete")
	parts, err := db.GetExportParts(exportId)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("failed to delete export")
	}

	for _, part := range parts {
		rctx.Log.Info("Locating datastore: " + part.DatastoreID)
		ds, err := datastore.LocateDatastore(rctx, part.DatastoreID)
		if err != nil {
			rctx.Log.Error(err)
			return api.InternalServerError("failed to delete export")
		}

		rctx.Log.Info("Deleting object: " + part.Location)
		err = ds.DeleteObject(part.Location)
		if err != nil {
			rctx.Log.Warn(err)
		}
	}

	rctx.Log.Info("Purging export from database")
	err = db.DeleteExportAndParts(exportId)
	if err != nil {
		rctx.Log.Error(err)
		return api.InternalServerError("failed to delete export")
	}

	return api.EmptyResponse{}
}
