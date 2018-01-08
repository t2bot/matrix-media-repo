package r0

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/client"
	"github.com/turt2live/matrix-media-repo/rcontext"
	"github.com/turt2live/matrix-media-repo/services"
	"github.com/turt2live/matrix-media-repo/util"
)

func ThumbnailMedia(w http.ResponseWriter, r *http.Request, i rcontext.RequestInfo) interface{} {
	if !ValidateUserCanDownload(r, i) {
		return client.AuthFailed()
	}

	params := mux.Vars(r)

	server := params["server"]
	mediaId := params["mediaId"]

	i.Log = i.Log.WithFields(logrus.Fields{
		"mediaId": mediaId,
		"server":  server,
	})

	widthStr := r.URL.Query().Get("width")
	heightStr := r.URL.Query().Get("height")
	method := r.URL.Query().Get("method")

	width := i.Config.Thumbnails.Sizes[0].Width
	height := i.Config.Thumbnails.Sizes[0].Height

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

	i.Log = i.Log.WithFields(logrus.Fields{
		"requestedWidth":  width,
		"requestedHeight": height,
		"requestedMethod": method,
	})

	mediaSvc := services.CreateMediaService(i)
	thumbSvc := services.CreateThumbnailService(i)

	media, err := mediaSvc.GetMedia(server, mediaId)
	if err != nil {
		if err == util.ErrMediaNotFound {
			return client.NotFoundError()
		} else if err == util.ErrMediaTooLarge {
			return client.RequestTooLarge()
		}
		i.Log.Error("Unexpected error locating media: " + err.Error())
		return client.InternalServerError("Unexpected Error")
	}

	thumb, err := thumbSvc.GetThumbnail(media, width, height, method)
	if err != nil {
		if err == util.ErrMediaTooLarge {
			i.Log.Warn("Media too large to thumbnail, returning source image instead")
			return &DownloadMediaResponse{
				ContentType: media.ContentType,
				SizeBytes:   media.SizeBytes,
				Location:    media.Location,
				Filename:    "thumbnail",
			}
		}
		i.Log.Error("Unexpected error getting thumbnail: " + err.Error())
		return client.InternalServerError("Unexpected Error")
	}

	return &DownloadMediaResponse{
		ContentType: thumb.ContentType,
		SizeBytes:   thumb.SizeBytes,
		Location:    thumb.Location,
		Filename:    "thumbnail",
	}
}
