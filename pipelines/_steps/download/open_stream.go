package download

import (
	"errors"
	"io"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/datastores"
	"github.com/turt2live/matrix-media-repo/redislib"
)

func OpenStream(ctx rcontext.RequestContext, media *database.Locatable) (io.ReadCloser, error) {
	reader, err := redislib.TryGetMedia(ctx, media.Sha256Hash)
	if err != nil || reader != nil {
		ctx.Log.Debugf("Got %s from cache", media.Sha256Hash)
		return io.NopCloser(reader), err
	}

	ds, ok := datastores.Get(ctx, media.DatastoreId)
	if !ok {
		return nil, errors.New("unable to locate datastore for media")
	}

	return datastores.Download(ctx, ds, media.Location)
}
