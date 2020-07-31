package thumbnail_controller

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/metrics"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/thumbnailing"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/resource_handler"
)

type thumbnailResourceHandler struct {
	resourceHandler *resource_handler.ResourceHandler
}

type thumbnailRequest struct {
	media    *types.Media
	width    int
	height   int
	method   string
	animated bool
}

type thumbnailResponse struct {
	thumbnail *types.Thumbnail
	err       error
}

type GeneratedThumbnail struct {
	ContentType       string
	DatastoreId       string
	DatastoreLocation string
	SizeBytes         int64
	Animated          bool
	Sha256Hash        string
}

var resHandlerInstance *thumbnailResourceHandler
var resHandlerSingletonLock = &sync.Once{}

func getResourceHandler() *thumbnailResourceHandler {
	if resHandlerInstance == nil {
		resHandlerSingletonLock.Do(func() {
			handler, err := resource_handler.New(config.Get().Thumbnails.NumWorkers, thumbnailWorkFn)
			if err != nil {
				panic(err)
			}

			resHandlerInstance = &thumbnailResourceHandler{handler}
		})
	}

	return resHandlerInstance
}

func thumbnailWorkFn(request *resource_handler.WorkRequest) interface{} {
	info := request.Metadata.(*thumbnailRequest)
	ctx := rcontext.Initial().LogWithFields(logrus.Fields{
		"worker_requestId": request.Id,
		"worker_media":     info.media.Origin + "/" + info.media.MediaId,
		"worker_width":     info.width,
		"worker_height":    info.height,
		"worker_method":    info.method,
		"worker_animated":  info.animated,
	})
	ctx.Log.Info("Processing thumbnail request")

	generated, err := GenerateThumbnail(info.media, info.width, info.height, info.method, info.animated, ctx)
	if err != nil {
		return &thumbnailResponse{err: err}
	}


	if info.animated != generated.Animated {
		ctx.Log.Warn("Animation state changed to ", generated.Animated)

		// Update the animation state to ensure that the record gets persisted with the right flag.
		generated.Animated = info.animated
	}

	newThumb := &types.Thumbnail{
		Origin:      info.media.Origin,
		MediaId:     info.media.MediaId,
		Width:       info.width,
		Height:      info.height,
		Method:      info.method,
		Animated:    generated.Animated,
		CreationTs:  util.NowMillis(),
		ContentType: generated.ContentType,
		DatastoreId: generated.DatastoreId,
		Location:    generated.DatastoreLocation,
		SizeBytes:   generated.SizeBytes,
		Sha256Hash:  generated.Sha256Hash,
	}

	db := storage.GetDatabase().GetThumbnailStore(ctx)
	err = db.Insert(newThumb)
	if err != nil {
		ctx.Log.Error("Unexpected error caching thumbnail: " + err.Error())
		return &thumbnailResponse{err: err}
	}

	return &thumbnailResponse{thumbnail: newThumb}
}

func (h *thumbnailResourceHandler) GenerateThumbnail(media *types.Media, width int, height int, method string, animated bool) chan *thumbnailResponse {
	resultChan := make(chan *thumbnailResponse)
	go func() {
		reqId := fmt.Sprintf("thumbnail_%s_%s_%d_%d_%s_%t", media.Origin, media.MediaId, width, height, method, animated)
		c := h.resourceHandler.GetResource(reqId, &thumbnailRequest{
			media:    media,
			width:    width,
			height:   height,
			method:   method,
			animated: animated,
		})
		defer close(c)
		result := <-c
		resultChan <- result.(*thumbnailResponse)
	}()
	return resultChan
}

func GenerateThumbnail(media *types.Media, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*GeneratedThumbnail, error) {
	allowAnimated := ctx.Config.Thumbnails.AllowAnimated
	animated = animated && allowAnimated

	mediaStream, err := datastore.DownloadStream(ctx, media.DatastoreId, media.Location)
	if err != nil {
		ctx.Log.Error("Error getting file: ", err)
		return nil, err
	}

	mediaContentType := util.FixContentType(media.ContentType)

	thumbImg, err := thumbnailing.GenerateThumbnail(mediaStream, mediaContentType, width, height, method, animated, ctx)
	if err != nil {
		ctx.Log.Error("Error generating thumbnail: ", err)
		return nil, err
	}

	metric := metrics.ThumbnailsGenerated.With(prometheus.Labels{
		"width":    strconv.Itoa(width),
		"height":   strconv.Itoa(height),
		"method":   method,
		"animated": strconv.FormatBool(animated),
		"origin":   media.Origin,
	})

	thumb := &GeneratedThumbnail{
		Animated: animated,
	}

	if thumbImg == nil {
		// Image is too small - don't upscale
		thumb.ContentType = mediaContentType
		thumb.DatastoreId = media.DatastoreId
		thumb.DatastoreLocation = media.Location
		thumb.SizeBytes = media.SizeBytes
		thumb.Sha256Hash = media.Sha256Hash
		ctx.Log.Warn("Image too small, returning raw image")
		metric.Inc()
		return thumb, nil
	}

	defer thumbImg.Reader.Close()
	b, err := ioutil.ReadAll(thumbImg.Reader)
	if err != nil {
		return nil, err
	}

	ds, err := datastore.PickDatastore(common.KindThumbnails, ctx)
	if err != nil {
		return nil, err
	}
	info, err := ds.UploadFile(ioutil.NopCloser(bytes.NewBuffer(b)), int64(len(b)), ctx)
	if err != nil {
		ctx.Log.Error("Unexpected error saving thumbnail: " + err.Error())
		return nil, err
	}

	thumb.Animated = thumbImg.Animated
	thumb.DatastoreLocation = info.Location
	thumb.DatastoreId = ds.DatastoreId
	thumb.ContentType = thumbImg.ContentType
	thumb.SizeBytes = info.SizeBytes
	thumb.Sha256Hash = info.Sha256Hash

	metric.Inc()
	return thumb, nil
}
