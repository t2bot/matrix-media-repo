package custom

import (
	"encoding/json"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/api/_routers"

	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/matrix"
)

func GetFederationInfo(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	serverName := _routers.GetParam("serverName", r)

	if !_routers.ServerNameRegex.MatchString(serverName) {
		return _responses.BadRequest("invalid server name")
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"serverName": serverName,
	})

	url, hostname, err := matrix.GetServerApiUrl(serverName)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(err.Error())
	}

	versionUrl := url + "/_matrix/federation/v1/version"
	versionResponse, err := matrix.FederatedGet(rctx, versionUrl, hostname, serverName, matrix.NoSigningKey)
	if versionResponse != nil {
		defer versionResponse.Body.Close()
	}
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(err.Error())
	}

	decoder := json.NewDecoder(versionResponse.Body)
	out := make(map[string]interface{})
	err = decoder.Decode(&out)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(err.Error())
	}

	resp := make(map[string]interface{})
	resp["base_url"] = url
	resp["hostname"] = hostname
	resp["versions_response"] = out
	return &_responses.DoNotCacheResponse{Payload: resp}
}
