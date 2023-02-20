package custom

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/api/_routers"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/matrix"
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
	versionResponse, err := matrix.FederatedGet(versionUrl, hostname, rctx)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(err.Error())
	}

	c, err := ioutil.ReadAll(versionResponse.Body)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError(err.Error())
	}

	out := make(map[string]interface{})
	err = json.Unmarshal(c, &out)
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
