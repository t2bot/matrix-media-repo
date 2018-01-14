package thumbnail_service

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/resource_handler"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type thumbnailResourceHandler struct {
	resourceHandler *resource_handler.ResourceHandler
}

type thumbnailRequest struct {
	media           *types.Media
	width           int
	height          int
	method          string
	animated        bool
	forceGeneration bool
}

type thumbnailResponse struct {
	thumbnail *types.Thumbnail
	err       error
}

var resHandlerInstance *thumbnailResourceHandler
var resHandlerSingletonLock = &sync.Once{}

func getResourceHandler() (*thumbnailResourceHandler) {
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
	log := logrus.WithFields(logrus.Fields{
		"worker_requestId":       request.Id,
		"worker_media":           info.media.Origin + "/" + info.media.MediaId,
		"worker_width":           info.width,
		"worker_height":          info.height,
		"worker_method":          info.method,
		"worker_animated":        info.animated,
		"worker_forceGeneration": info.forceGeneration,
	})
	log.Info("Processing thumbnail request")

	ctx := context.TODO() // TODO: Should we use a real context?

	thumbnailer := NewThumbnailer(ctx, log)
	generated, err := thumbnailer.GenerateThumbnail(info.media, info.width, info.height, info.method, info.animated, info.forceGeneration)
	if err != nil {
		return &thumbnailResponse{err: err}
	}

	svc := New(ctx, log) // thumbnail_service (us)

	newThumb := &types.Thumbnail{
		Origin:      info.media.Origin,
		MediaId:     info.media.MediaId,
		Width:       info.width,
		Height:      info.height,
		Method:      info.method,
		Animated:    generated.Animated,
		CreationTs:  util.NowMillis(),
		ContentType: generated.ContentType,
		Location:    generated.DiskLocation,
		SizeBytes:   generated.SizeBytes,
	}

	err = svc.store.Insert(newThumb)
	if err != nil {
		log.Error("Unexpected error caching thumbnail: " + err.Error())
		return &thumbnailResponse{err: err}
	}

	return &thumbnailResponse{thumbnail: newThumb}
}

func (h *thumbnailResourceHandler) GenerateThumbnail(media *types.Media, width int, height int, method string, animated bool, forceGeneration bool) chan *thumbnailResponse {
	resultChan := make(chan *thumbnailResponse)
	go func() {
		reqId := fmt.Sprintf("thumbnail_%s_%s_%d_%d_%s_%t_%t", media.Origin, media.MediaId, width, height, method, animated, forceGeneration)
		result := <-h.resourceHandler.GetResource(reqId, &thumbnailRequest{
			media:           media,
			width:           width,
			height:          height,
			method:          method,
			animated:        animated,
			forceGeneration: forceGeneration,
		})
		resultChan <- result.(*thumbnailResponse)
	}()
	return resultChan
}
