package matrix

import "github.com/t2bot/matrix-media-repo/common/rcontext"

type ClientVersionsResponse struct {
	Versions         []string        `json:"versions"`
	UnstableFeatures map[string]bool `json:"unstable_features"`
}

func ClientVersions(ctx rcontext.RequestContext, serverName string, accessToken string, appserviceUserId string, ipAddr string) (*ClientVersionsResponse, error) {
	response := &ClientVersionsResponse{}
	err := doBreakerRequest(ctx, serverName, accessToken, appserviceUserId, ipAddr, "GET", "/_matrix/client/versions", response)
	if err != nil {
		return nil, err
	}
	return response, nil
}
