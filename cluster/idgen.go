package cluster

import (
	"errors"
	"fmt"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
	"io/ioutil"
	"net/http"
	"time"
)

func GetId() (string, error) {
	req, err := http.NewRequest("GET", util.MakeUrl(config.Get().Cluster.IDGenerator.Location, "/v1/id"), nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Get().Cluster.IDGenerator.Secret))

	client := &http.Client{
		Timeout: 1 * time.Second, // the server should be fast (much faster than this)
	}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer cleanup.DumpAndCloseStream(res.Body)

	contents, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", errors.New(fmt.Sprintf("unexpected status code from ID generator: %d", res.StatusCode))
	}

	return string(contents), nil
}
