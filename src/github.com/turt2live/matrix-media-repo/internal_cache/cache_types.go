package internal_cache

import (
	"bytes"

	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type cachedFile struct {
	media     *types.Media
	thumbnail *types.Thumbnail
	Contents  *bytes.Buffer
}

type cooldown struct {
	isEviction bool
	expiresTs  int64
}

func (c *cooldown) IsExpired() bool {
	return util.NowMillis() >= c.expiresTs
}
