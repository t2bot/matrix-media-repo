package r0

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/client"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/media_handler"
	"github.com/turt2live/matrix-media-repo/storage"
)

func ThumbnailMedia(w http.ResponseWriter, r *http.Request, db storage.Database, c config.MediaRepoConfig, log *logrus.Entry) interface{} {
	if !ValidateUserCanDownload(r, db, c) {
		return client.AuthFailed()
	}

	params := mux.Vars(r)

	server := params["server"]
	mediaId := params["mediaId"]

	log = log.WithFields(logrus.Fields{
		"mediaId": mediaId,
		"server":  server,
	})

	widthStr := r.URL.Query().Get("width")
	heightStr := r.URL.Query().Get("height")
	method := r.URL.Query().Get("method")

	width := c.Thumbnails.Sizes[0].Width
	height := c.Thumbnails.Sizes[0].Height

	if widthStr != "" {
		parsedWidth, err := strconv.Atoi(widthStr)
		if err != nil {
			return client.InternalServerError("Width does not appear to be an integer")
		}
		width = parsedWidth
	}
	if heightStr != "" {
		parsedHeight, err := strconv.Atoi(heightStr)
		if err != nil {
			return client.InternalServerError("Height does not appear to be an integer")
		}
		height = parsedHeight
	}
	if method == "" {
		method = "crop"
	}

	log = log.WithFields(logrus.Fields{
		"requestedWidth":  width,
		"requestedHeight": height,
		"requestedMethod": method,
	})

	media, err := media_handler.FindMedia(r.Context(), server, mediaId, c, db, log)
	if err != nil {
		if err == media_handler.ErrMediaNotFound {
			return client.NotFoundError()
		} else if err == media_handler.ErrMediaTooLarge {
			return client.RequestTooLarge()
		}
		log.Error("Unexpected error locating media: " + err.Error())
		return client.InternalServerError("Unexpected Error")
	}

	thumb, err := media_handler.GetThumbnail(r.Context(), media, width, height, method, c, db, log)
	if err != nil {
		log.Error("Unexpected error getting thumbnail: " + err.Error())
		return client.InternalServerError("Unexpected Error")
	}

	return &DownloadMediaResponse{
		ContentType: thumb.ContentType,
		SizeBytes:   thumb.SizeBytes,
		Location:    thumb.Location,
		Filename:    "thumbnail",
	}
}
