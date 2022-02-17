package ids

import (
	"github.com/turt2live/matrix-media-repo/cluster"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/util"
	"strconv"
)

func NewUniqueId() (string, error) {
	if config.Get().Cluster.IDGenerator.Secret != "" {
		return cluster.GetId()
	}

	b, err := util.GenerateRandomBytes(64)
	if err != nil {
		return "", err
	}
	return util.GetSha1OfString(string(b) + strconv.FormatInt(util.NowMillis(), 10))
}
