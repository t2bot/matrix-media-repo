package r0

import (
	"net/http"

	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/storage"
)



// Request:
//   Path params: {serverName}, {mediaId}
//   QS: ?width=&height=&method=
//       "method" can be crop or scale
//
// Response:
//   Headers: Content-Type
//   Body: <byte[]>

func ThumbnailMedia(w http.ResponseWriter, r *http.Request, db storage.Database, c config.MediaRepoConfig) interface{} {
	return nil
}
