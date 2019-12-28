package unstable

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/disintegration/imaging"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/download_controller"
	"github.com/turt2live/matrix-media-repo/storage"
)

type mediaInfoHashes struct {
	Sha256 string `json:"sha256"`
}

type mediaInfoThumbnail struct {
	Width  int  `json:"width"`
	Height int  `json:"height"`
	Ready  bool `json:"ready"`
}

type MediaInfoResponse struct {
	ContentUri  string                `json:"content_uri"`
	ContentType string                `json:"content_type"`
	Width       int                   `json:"width,omitempty"`
	Height      int                   `json:"height,omitempty"`
	Size        int64                 `json:"size"`
	Hashes      mediaInfoHashes       `json:"hashes"`
	Thumbnails  []*mediaInfoThumbnail `json:"thumbnails,omitempty"`
}

func MediaInfo(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	params := mux.Vars(r)

	server := params["server"]
	mediaId := params["mediaId"]
	allowRemote := r.URL.Query().Get("allow_remote")

	downloadRemote := true
	if allowRemote != "" {
		parsedFlag, err := strconv.ParseBool(allowRemote)
		if err != nil {
			return api.InternalServerError("allow_remote flag does not appear to be a boolean")
		}
		downloadRemote = parsedFlag
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"mediaId":     mediaId,
		"server":      server,
		"allowRemote": downloadRemote,
	})

	streamedMedia, err := download_controller.GetMedia(server, mediaId, downloadRemote, true, rctx)
	if err != nil {
		if err == common.ErrMediaNotFound {
			return api.NotFoundError()
		} else if err == common.ErrMediaTooLarge {
			return api.RequestTooLarge()
		} else if err == common.ErrMediaQuarantined {
			return api.NotFoundError() // We lie for security
		}
		rctx.Log.Error("Unexpected error locating media: " + err.Error())
		return api.InternalServerError("Unexpected Error")
	}
	defer streamedMedia.Stream.Close()

	response := &MediaInfoResponse{
		ContentUri:  streamedMedia.KnownMedia.MxcUri(),
		ContentType: streamedMedia.KnownMedia.ContentType,
		Size:        streamedMedia.KnownMedia.SizeBytes,
		Hashes: mediaInfoHashes{
			Sha256: streamedMedia.KnownMedia.Sha256Hash,
		},
	}

	img, err := imaging.Decode(streamedMedia.Stream)
	if err == nil {
		response.Width = img.Bounds().Max.X
		response.Height = img.Bounds().Max.Y
	}

	thumbsDb := storage.GetDatabase().GetThumbnailStore(rctx)
	thumbs, err := thumbsDb.GetAllForMedia(streamedMedia.KnownMedia.Origin, streamedMedia.KnownMedia.MediaId)
	if err != nil && err != sql.ErrNoRows {
		rctx.Log.Error("Unexpected error locating media: " + err.Error())
		return api.InternalServerError("Unexpected Error")
	}

	if thumbs != nil && len(thumbs) > 0 {
		infoThumbs := make([]*mediaInfoThumbnail, 0)
		for _, thumb := range thumbs {
			infoThumbs = append(infoThumbs, &mediaInfoThumbnail{
				Width:  thumb.Width,
				Height: thumb.Height,
				Ready:  true,
			})
		}
		response.Thumbnails = infoThumbs
	}

	return response
}
