package r0

import (
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/client"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/services/media_service"
	"github.com/turt2live/matrix-media-repo/services/thumbnail_service"
	"github.com/turt2live/matrix-media-repo/util/errs"
)

func ThumbnailMedia(w http.ResponseWriter, r *http.Request, log *logrus.Entry) interface{} {
	if !ValidateUserCanDownload(r) {
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
	animatedStr := r.URL.Query().Get("animated")

	width := config.Get().Thumbnails.Sizes[0].Width
	height := config.Get().Thumbnails.Sizes[0].Height
	animated := false

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
	if animatedStr != "" {
		parsedFlag, err := strconv.ParseBool(animatedStr)
		if err != nil {
			return client.InternalServerError("Animated flag does not appear to be a boolean")
		}
		animated = parsedFlag
	}
	if method == "" {
		method = "scale"
	}

	log = log.WithFields(logrus.Fields{
		"requestedWidth":    width,
		"requestedHeight":   height,
		"requestedMethod":   method,
		"requestedAnimated": animated,
	})

	mediaSvc := media_service.New(r.Context(), log)
	thumbSvc := thumbnail_service.New(r.Context(), log)

	media, err := mediaSvc.GetMedia(server, mediaId)
	if err != nil {
		if err == errs.ErrMediaNotFound {
			return client.NotFoundError()
		} else if err == errs.ErrMediaTooLarge {
			return client.RequestTooLarge()
		}
		log.Error("Unexpected error locating media: " + err.Error())
		return client.InternalServerError("Unexpected Error")
	}

	thumb, err := thumbSvc.GetThumbnail(media, width, height, method, animated)
	if err != nil {
		fstream, errF := os.Open(media.Location)
		if errF != nil {
			log.Error("Unexpected error opening media: " + errF.Error())
			return client.InternalServerError("Unexpected Error")
		}

		if err == errs.ErrMediaTooLarge {
			log.Warn("Media too large to thumbnail, returning source image instead")
			return &DownloadMediaResponse{
				ContentType: media.ContentType,
				SizeBytes:   media.SizeBytes,
				Data:        fstream,
				Filename:    "thumbnail",
			}
		}
		log.Error("Unexpected error getting thumbnail: " + err.Error())
		return client.InternalServerError("Unexpected Error")
	}

	fstream, err := os.Open(thumb.Location)
	if err != nil {
		log.Error("Unexpected error opening thumbnail media: " + err.Error())
		return client.InternalServerError("Unexpected Error")
	}

	return &DownloadMediaResponse{
		ContentType: thumb.ContentType,
		SizeBytes:   thumb.SizeBytes,
		Data:        fstream,
		Filename:    "thumbnail",
	}
}
