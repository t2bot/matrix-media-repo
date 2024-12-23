package unstable

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/api/_apimeta"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/api/_routers"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/pipelines/pipeline_download"
	"github.com/t2bot/matrix-media-repo/thumbnailing"
	"github.com/t2bot/matrix-media-repo/thumbnailing/i"
	"github.com/t2bot/matrix-media-repo/util"
)

type mediaInfoHashes struct {
	Sha256 string `json:"sha256"`
}

type mediaInfoThumbnail struct {
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Ready       bool   `json:"ready"`
	ContentType string `json:"content_type,omitempty"`
	SizeBytes   int64  `json:"size,omitempty"`
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

	if util.IsHostIgnored(server) && !util.IsGlobalAdmin(user.UserId) {
		rctx.Log.Warn("Request blocked due to domain being ignored.")
		return _responses.MediaBlocked()
	}

	record, stream, err := pipeline_download.Execute(rctx, server, mediaId, pipeline_download.DownloadOpts{
		FetchRemoteIfNeeded: downloadRemote,
		BlockForReadUntil:   30 * time.Second,
		RecordOnly:          false,
	})
	// Error handling copied from download endpoint
	if err != nil {
		if errors.Is(err, common.ErrMediaNotFound) {
			return _responses.NotFoundError()
		} else if errors.Is(err, common.ErrMediaTooLarge) {
			return _responses.RequestTooLarge()
		} else if errors.Is(err, common.ErrMediaQuarantined) {
			rctx.Log.Debug("Quarantined media accessed. Has stream? ", stream != nil)
			if stream != nil {
				return _responses.MakeQuarantinedImageResponse(stream)
			} else {
				return _responses.NotFoundError() // We lie for security
			}
		} else if errors.Is(err, common.ErrMediaNotYetUploaded) {
			return _responses.NotYetUploaded()
		}
		rctx.Log.Error("Unexpected error locating media: ", err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected Error")
	}

	response := &MediaInfoResponse{
		ContentUri:  util.MxcUri(record.Origin, record.MediaId),
		ContentType: record.ContentType,
		Size:        record.SizeBytes,
		Hashes: mediaInfoHashes{
			Sha256: record.Sha256Hash,
		},
	}

	if strings.HasPrefix(response.ContentType, "image/") {
		img, err := imaging.Decode(stream)
		if err == nil {
			response.Width = img.Bounds().Max.X
			response.Height = img.Bounds().Max.Y
		}
	} else if strings.HasPrefix(response.ContentType, "audio/") {
		generator, reconstructed, err := thumbnailing.GetGenerator(stream, response.ContentType, false)
		if err == nil {
			if audiogenerator, ok := generator.(i.AudioGenerator); ok {
				audioInfo, err := audiogenerator.GetAudioData(reconstructed, 768, rctx)
				if err == nil {
					response.KeySamples = audioInfo.KeySamples
					response.NumChannels = audioInfo.Channels
					response.DurationSeconds = audioInfo.Duration.Seconds()
					response.NumTotalSamples = audioInfo.TotalSamples
				}
			}
		}
	}

	thumbs, err := database.GetInstance().Thumbnails.Prepare(rctx).GetForMedia(record.Origin, record.MediaId)
	if err != nil {
		rctx.Log.Error("Unexpected error locating media thumbnails: ", err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Unexpected Error")
	}

	if len(thumbs) > 0 {
		infoThumbs := make([]*mediaInfoThumbnail, 0)
		for _, thumb := range thumbs {
			infoThumbs = append(infoThumbs, &mediaInfoThumbnail{
				Width:       thumb.Width,
				Height:      thumb.Height,
				Ready:       true,
				ContentType: thumb.ContentType,
				SizeBytes:   thumb.SizeBytes,
			})
		}
		response.Thumbnails = infoThumbs
	}

	return response
}
