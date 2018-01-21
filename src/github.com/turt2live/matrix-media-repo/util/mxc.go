package util

import (
	"strings"

	"github.com/pkg/errors"
)

func SplitMxc(mxc string) (string, string, error) {
	if strings.Index(mxc, "mxc://") != 0 {
		return "", "", errors.New("not a valid mxc uri: missing protocol")
	}

	mxc = mxc[6:]                    // remove protocol
	mxc = strings.Split(mxc, "?")[0] // take off any query string

	parts := strings.Split(mxc, "/")
	if len(parts) != 2 {
		return "", "", errors.New("not a valid mxc uri: not in the format of mxc://origin/media_id")
	}

	return parts[0], parts[1], nil // origin, media id
}
