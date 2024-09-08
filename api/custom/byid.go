package custom

import (
	"net/http"

	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/api/_routers"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/datastores"
	"github.com/t2bot/matrix-media-repo/pipelines/_steps/download"
)

func GetMediaById(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	if !user.IsShared {
		return _responses.AuthFailed()
	}

	// TODO: This is beyond dangerous and needs proper filtering

	db := database.GetInstance().Media.Prepare(rctx)
	ds, err := datastores.Pick(rctx, datastores.LocalMediaKind)
	if err != nil {
		panic(err)
	}
	objectId := _routers.GetParam("objectId", r)
	medias, err := db.GetByLocation(ds.Id, objectId)
	if err != nil {
		panic(err)
	}

	media := medias[0]
	stream, err := download.OpenStream(rctx, media.Locatable)
	if err != nil {
		panic(err)
	}

	return &_responses.DownloadResponse{
		ContentType:       media.ContentType,
		Filename:          media.UploadName,
		SizeBytes:         media.SizeBytes,
		Data:              stream,
		TargetDisposition: "infer",
	}
}
