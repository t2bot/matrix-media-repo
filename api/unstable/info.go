package unstable

import (
	"bytes"
	"database/sql"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/api/_routers"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/stream_util"

	"github.com/disintegration/imaging"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/download_controller"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/thumbnailing"
	"github.com/turt2live/matrix-media-repo/thumbnailing/i"
	"github.com/turt2live/matrix-media-repo/util/util_byte_seeker"
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
	ContentUri      string                `json:"content_uri"`
	ContentType     string                `json:"content_type"`
	Width           int                   `json:"width,omitempty"`
	Height          int                   `json:"height,omitempty"`
	Size            int64                 `json:"size"`
	Hashes          mediaInfoHashes       `json:"hashes"`
	Thumbnails      []*mediaInfoThumbnail `json:"thumbnails,omitempty"`
	DurationSeconds float64               `json:"duration,omitempty"`
	NumTotalSamples int                   `json:"num_total_samples,omitempty"`
	KeySamples      [][2]float64          `json:"key_samples,omitempty"`
	NumChannels     int                   `json:"num_channels,omitempty"`
}

func MediaInfo(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	server := _routers.GetParam("server", r)
	mediaId := _routers.GetParam("mediaId", r)
	allowRemote := r.URL.Query().Get("allow_remote")

	if !_routers.ServerNameRegex.MatchString(server) {
		return _responses.BadRequest("invalid server ID")
	}

	downloadRemote := true
	if allowRemote != "" {
		parsedFlag, err := strconv.ParseBool(allowRemote)
		if err != nil {
			return _responses.InternalServerError("allow_remote flag does not appear to be a boolean")
		}
		downloadRemote = parsedFlag
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"mediaId":     mediaId,
		"server":      server,
		"allowRemote": downloadRemote,
	})

	if !util.IsGlobalAdmin(user.UserId) && util.IsHostIgnored(server) {
		rctx.Log.Warn("Request blocked due to domain being ignored.")
		return _responses.MediaBlocked()
	}

	streamedMedia, err := download_controller.GetMedia(server, mediaId, downloadRemote, true, rctx)
	if err != nil {
		if err == common.ErrMediaNotFound {
			return _responses.NotFoundError()
		} else if err == common.ErrMediaTooLarge {
			return _responses.RequestTooLarge()
		} else if err == common.ErrMediaQuarantined {
			return _responses.NotFoundError() // We lie for security
		}
		rctx.Log.Error("Unexpected error locating media: " + err.Error())
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected Error")
	}
	defer stream_util.DumpAndCloseStream(streamedMedia.Stream)

	b, err := ioutil.ReadAll(streamedMedia.Stream)
	if err != nil {
		rctx.Log.Error("Unexpected error processing media: " + err.Error())
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected Error")
	}

	response := &MediaInfoResponse{
		ContentUri:  streamedMedia.KnownMedia.MxcUri(),
		ContentType: streamedMedia.KnownMedia.ContentType,
		Size:        streamedMedia.KnownMedia.SizeBytes,
		Hashes: mediaInfoHashes{
			Sha256: streamedMedia.KnownMedia.Sha256Hash,
		},
	}

	img, err := imaging.Decode(bytes.NewBuffer(b))
	if err == nil {
		response.Width = img.Bounds().Max.X
		response.Height = img.Bounds().Max.Y
	}

	thumbsDb := storage.GetDatabase().GetThumbnailStore(rctx)
	thumbs, err := thumbsDb.GetAllForMedia(streamedMedia.KnownMedia.Origin, streamedMedia.KnownMedia.MediaId)
	if err != nil && err != sql.ErrNoRows {
		rctx.Log.Error("Unexpected error locating media: " + err.Error())
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected Error")
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

	if strings.HasPrefix(response.ContentType, "audio/") {
		generator, err := thumbnailing.GetGenerator(util_byte_seeker.NewByteSeeker(b), response.ContentType, false)
		if err == nil {
			if audiogenerator, ok := generator.(i.AudioGenerator); ok {
				audioInfo, err := audiogenerator.GetAudioData(b, 768, rctx)
				if err == nil {
					response.KeySamples = audioInfo.KeySamples
					response.NumChannels = audioInfo.Channels
					response.DurationSeconds = audioInfo.Duration.Seconds()
					response.NumTotalSamples = audioInfo.TotalSamples
				}
			}
		}
	}

	return response
}
