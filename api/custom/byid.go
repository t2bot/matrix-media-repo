package custom

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/api/_routers"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/datastores"
	"github.com/t2bot/matrix-media-repo/pipelines/_steps/download"
	"github.com/t2bot/matrix-media-repo/util"
)

func GetMediaById(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	//if !user.IsShared {
	//	return _responses.AuthFailed()
	//}

	// TODO: This is beyond dangerous and needs proper filtering

	// Parse the `request` to ensure we actually sent this request
	requestVal := r.URL.Query().Get("request")
	requestValParts := strings.Split(requestVal, ".")
	if len(requestValParts) != 2 {
		rctx.Log.Error("Need exactly 2 parts for `request`")
		return _responses.AuthFailed()
	}
	verifyMac := requestValParts[0]
	toUrlB, err := hex.DecodeString(requestValParts[1])
	if err != nil {
		rctx.Log.Error("Failed to decode request value:", err)
		return _responses.AuthFailed()
	}
	toUrl := string(toUrlB)
	mac := hmac.New(sha512.New, []byte("THIS IS ANOTHER SECRET VALUE")) // TODO: @@ Actual secret key
	mac.Write([]byte(toUrl))
	expectedMac := hex.EncodeToString(mac.Sum(nil))
	if strings.ToLower(verifyMac) != strings.ToLower(expectedMac) {
		return _responses.AuthFailed()
	}

	// Verify the HMAC from the worker too
	query := r.URL.Query()
	suppliedHmac := query.Get("verify")
	query.Del("verify")
	r.URL.RawQuery = query.Encode()
	r.URL.Host = r.Host                                        // TODO: Why is this unset??
	r.URL.Scheme = "https"                                     // TODO: Why is this unset??
	mac = hmac.New(sha512.New, []byte("THIS_IS_A_SECRET_KEY")) // TODO: @@ Actual secret key
	rctx.Log.Info("URL: ", r.URL.String())
	mac.Write([]byte(r.URL.String()))
	expectedMac = hex.EncodeToString(mac.Sum(nil))
	if strings.ToLower(suppliedHmac) != strings.ToLower(expectedMac) {
		rctx.Log.Error("HMAC mismatch")
		return _responses.AuthFailed()
	}

	// Verify that the path for the `request` is the same as our called path
	parsedUrl, err := url.Parse(toUrl)
	if err != nil {
		rctx.Log.Error("Failed to parse URL:", err)
		return _responses.AuthFailed()
	}
	if parsedUrl.Path != r.URL.Path {
		rctx.Log.Error("Wrong path or query")
		return _responses.AuthFailed()
	}

	// Verify that the original request isn't expired
	expVal := parsedUrl.Query().Get("exp")
	if expVal != "" {
		exp, err := strconv.ParseInt(expVal, 10, 64)
		if err != nil {
			rctx.Log.Error("Failed to parse exp:", err)
			return _responses.AuthFailed()
		}
		if exp <= util.NowMillis() {
			rctx.Log.Error("Request expired")
			return _responses.AuthFailed()
		}
	}

	// ---- request verified - we can now serve the media ----

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
