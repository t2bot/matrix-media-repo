package media_cache

import (
	"bytes"
	"context"

	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/download_tracker"
)

type cachedFile struct {
	media     *types.Media
	thumbnail *types.Thumbnail
	contents  *bytes.Buffer
}

type cooldown struct {
	isEviction bool
	expiresTs  int64
}

type mediaCacheFactory struct {
	cache         *cache.Cache
	cooldownCache *cache.Cache
	tracker       *download_tracker.DownloadTracker
	size          int64
}

type mediaCache struct {
	cache         *cache.Cache
	cooldownCache *cache.Cache
	tracker       *download_tracker.DownloadTracker
	size          int64
	log           *logrus.Entry
	ctx           context.Context
}

func (c *cooldown) IsExpired() bool {
	return util.NowMillis() >= c.expiresTs
}
