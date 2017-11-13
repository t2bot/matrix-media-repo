package r0

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/turt2live/matrix-media-repo/client"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/media_handler"
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
	params := mux.Vars(r)

	server := params["server"]
	mediaId := params["mediaId"]

	widthStr := r.URL.Query().Get("width")
	heightStr := r.URL.Query().Get("height")
	method := r.URL.Query().Get("method")

	width := c.Thumbnails.Sizes[0].Width
	height := c.Thumbnails.Sizes[0].Height

	if widthStr != "" {
		parsedWidth, err := strconv.Atoi(widthStr)
		if err != nil {
			return client.InternalServerError(err.Error())
		}
		width = parsedWidth
	}
	if heightStr != "" {
		parsedHeight, err := strconv.Atoi(heightStr)
		if err != nil {
			return client.InternalServerError(err.Error())
		}
		height = parsedHeight
	}
	if method == "" {
		method = "crop"
	}

	media, err := media_handler.FindMedia(r.Context(), server, mediaId, c, db)
	if err != nil {
		if err == media_handler.ErrMediaNotFound {
			return client.NotFoundError()
		}
		return client.InternalServerError(err.Error())
	}

	thumb, err := media_handler.GetThumbnail(r.Context(), media, width, height, method, c, db)
	if err != nil {
		return client.InternalServerError(err.Error())
	}

	return &DownloadMediaResponse{
		ContentType: thumb.ContentType,
		SizeBytes:   thumb.SizeBytes,
		Location:    thumb.Location,
		Filename:    "thumbnail",
	}
}
