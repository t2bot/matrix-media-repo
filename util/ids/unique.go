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

	s, err := util.GenerateRandomString(64)
	if err != nil {
		return "", err
	}
	return util.GetSha1OfString(s + strconv.FormatInt(util.NowMillis(), 10))
}
