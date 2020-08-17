package internal_cache

import (
	"io"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/types"
)

func StreamerForMedia(media *types.Media) FetchFunction {
	return func() (io.ReadCloser, error) {
		return datastore.DownloadStream(rcontext.Initial(), media.DatastoreId, media.Location)
	}
}

func StreamerForThumbnail(media *types.Thumbnail) FetchFunction {
	return func() (io.ReadCloser, error) {
		return datastore.DownloadStream(rcontext.Initial(), media.DatastoreId, media.Location)
	}
}
