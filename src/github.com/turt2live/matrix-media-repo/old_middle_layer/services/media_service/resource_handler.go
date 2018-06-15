package media_service

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/old_middle_layer/resource_handler"
	"github.com/turt2live/matrix-media-repo/types"
)

type mediaResourceHandler struct {
	resourceHandler *resource_handler.ResourceHandler
}

type downloadRequest struct {
	origin  string
	mediaId string
}

type downloadResponse struct {
	media *types.Media
	err   error
}

var resHandlerInstance *mediaResourceHandler
var resHandlerSingletonLock = &sync.Once{}

func getResourceHandler() (*mediaResourceHandler) {
	if resHandlerInstance == nil {
		resHandlerSingletonLock.Do(func() {
			handler, err := resource_handler.New(config.Get().Downloads.NumWorkers, downloadResourceWorkFn)
			if err != nil {
				panic(err)
			}

			resHandlerInstance = &mediaResourceHandler{handler}
		})
	}

	return resHandlerInstance
}

func downloadResourceWorkFn(request *resource_handler.WorkRequest) interface{} {
	info := request.Metadata.(*downloadRequest)
	log := logrus.WithFields(logrus.Fields{
		"worker_requestId":      request.Id,
		"worker_requestOrigin":  info.origin,
		"worker_requestMediaId": info.mediaId,
	})
	log.Info("Downloading remote media")

	ctx := context.TODO() // TODO: Should we use a real context?

	downloader := newRemoteMediaDownloader(ctx, log)
	downloaded, err := downloader.Download(info.origin, info.mediaId)
	if err != nil {
		return &downloadResponse{err: err}
	}

	svc := New(ctx, log) // media_service (us)
	defer downloaded.Contents.Close()

	media, err := svc.StoreMedia(downloaded.Contents, downloaded.ContentType, downloaded.DesiredFilename, "", info.origin, info.mediaId)
	if err != nil {
		return &downloadResponse{err: err}
	}

	return &downloadResponse{media, err}
}

func (h *mediaResourceHandler) DownloadRemoteMedia(origin string, mediaId string) chan *downloadResponse {
	resultChan := make(chan *downloadResponse)
	go func() {
		reqId := "remote_download:" + origin + "_" + mediaId
		result := <-h.resourceHandler.GetResource(reqId, &downloadRequest{origin, mediaId})
		resultChan <- result.(*downloadResponse)
	}()
	return resultChan
}
