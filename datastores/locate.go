package datastores

import (
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

func Get(ctx rcontext.RequestContext, dsId string) (config.DatastoreConfig, bool) {
	for _, c := range ctx.Config.DataStores {
		if c.Id == dsId {
			return c, true
		}
	}
	return config.DatastoreConfig{}, false
}
