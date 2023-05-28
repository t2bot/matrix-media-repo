package datastores

import (
	"errors"

	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
)

func Pick(ctx rcontext.RequestContext, kind Kind) (config.DatastoreConfig, error) {
	usable := make([]config.DatastoreConfig, 0)
	for _, conf := range ctx.Config.DataStores {
		if !HasListedKind(conf.MediaKinds, kind) {
			continue
		}
		usable = append(usable, conf)
	}

	if len(usable) < 0 {
		return config.DatastoreConfig{}, errors.New("unable to locate a usable datastore")
	}
	if len(usable) == 1 {
		return usable[0], nil
	}

	// Find the smallest datastore, by relative size
	dsSize := int64(-1)
	idx := 0
	db := database.GetInstance().MetadataView.Prepare(ctx)
	for i, ds := range usable {
		size, err := db.EstimateDatastoreSize(ds.Id)
		if err != nil {
			return config.DatastoreConfig{}, err
		}
		if dsSize < 0 || size > dsSize {
			idx = i
		}
	}
	return usable[idx], nil
}
