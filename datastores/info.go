package datastores

import (
	"errors"
	"fmt"

	"github.com/turt2live/matrix-media-repo/common/config"
)

func GetUri(ds config.DatastoreConfig) (string, error) {
	if ds.Type == "s3" {
		s3c, err := getS3(ds)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("s3://%s/%s", s3c.client.EndpointURL().Hostname(), s3c.bucket), nil
	} else if ds.Type == "file" {
		return ds.Options["path"], nil
	} else {
		return "", errors.New("unknown datastore type - contact developer")
	}
}
