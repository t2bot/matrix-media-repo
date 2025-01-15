package custom

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/api/_routers"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/datastores"
	"github.com/t2bot/matrix-media-repo/pipelines/_steps/download"
)

func GetMediaById(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	//if !user.IsShared {
	//	return _responses.AuthFailed()
	//}

	// TODO: This is beyond dangerous and needs proper filtering

	requestVal := r.URL.Query().Get("request")
	requestValParts := strings.Split(requestVal, ".")
	if len(requestValParts) != 2 {
		return _responses.AuthFailed()
	}
	verifyMac := requestValParts[0]
	toUrlB, err := hex.DecodeString(requestValParts[1])
	if err != nil {
		rctx.Log.Error("Failed to decode request value: %s", err)
		return _responses.AuthFailed()
	}
	toUrl := string(toUrlB)
	mac := hmac.New(sha512.New, []byte("THIS IS ANOTHER SECRET VALUE")) // TODO: @@ Actual secret key
	mac.Write([]byte(toUrl))
	expectedMac := hex.EncodeToString(mac.Sum(nil))
	if strings.ToLower(verifyMac) != strings.ToLower(expectedMac) {
		return _responses.AuthFailed()
	}

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
