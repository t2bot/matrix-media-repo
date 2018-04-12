package url_service

import (
	"context"
	"fmt"
	"sync"

	"github.com/disintegration/imaging"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/resource_handler"
	"github.com/turt2live/matrix-media-repo/services/media_service"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type urlResourceHandler struct {
	resourceHandler *resource_handler.ResourceHandler
}

type urlPreviewRequest struct {
	urlStr    string
	forUserId string
	onHost    string
}

type urlPreviewResponse struct {
	preview *types.UrlPreview
	err     error
}

var resHandlerInstance *urlResourceHandler
var resHandlerSingletonLock = &sync.Once{}

func getResourceHandler() (*urlResourceHandler) {
	if resHandlerInstance == nil {
		resHandlerSingletonLock.Do(func() {
			handler, err := resource_handler.New(config.Get().UrlPreviews.NumWorkers, urlPreviewWorkFn)
			if err != nil {
				panic(err)
			}

			resHandlerInstance = &urlResourceHandler{handler}
		})
	}

	return resHandlerInstance
}

func urlPreviewWorkFn(request *resource_handler.WorkRequest) interface{} {
	info := request.Metadata.(*urlPreviewRequest)
	log := logrus.WithFields(logrus.Fields{
		"worker_requestId": request.Id,
		"worker_url":       info.urlStr,
		"worker_previewer": "OpenGraph",
	})
	log.Info("Processing url preview request")

	ctx := context.TODO() // TODO: Should we use a real context?

	svc := New(ctx, log) // url_service (us)
	previewer := NewOpenGraphPreviewer(ctx, log)
	preview, err := previewer.GeneratePreview(info.urlStr)
	if err == ErrPreviewUnsupported {
		log.Info("OpenGraph preview for this URL is unsupported - treating it as a file")
		log = log.WithFields(logrus.Fields{"worker_previewer": "File"})

		filePreviewer := NewFilePreviewer(ctx, log)
		preview, err = filePreviewer.GeneratePreview(info.urlStr)
	}
	if err != nil {
		// Transparently convert "unsupported" to "not found" for processing
		if err == ErrPreviewUnsupported {
			err = common.ErrMediaNotFound
		}

		if err == common.ErrMediaNotFound {
			svc.store.InsertPreviewError(info.urlStr, common.ErrCodeNotFound)
		} else {
			svc.store.InsertPreviewError(info.urlStr, common.ErrCodeUnknown)
		}
		return &urlPreviewResponse{err: err}
	}

	result := &types.UrlPreview{
		Url:         preview.Url,
		SiteName:    preview.SiteName,
		Type:        preview.Type,
		Description: preview.Description,
		Title:       preview.Title,
	}

	// Store the thumbnail, if there is one
	mediaSvc := media_service.New(ctx, log)
	if preview.Image != nil && !mediaSvc.IsTooLarge(preview.Image.ContentLength, preview.Image.ContentLengthHeader) {
		// UploadMedia will close the read stream for the thumbnail and dedupe the image
		media, err := mediaSvc.UploadMedia(preview.Image.Data, preview.Image.ContentType, preview.Image.Filename, info.forUserId, info.onHost)
		if err != nil {
			log.Warn("Non-fatal error storing preview thumbnail: " + err.Error())
		} else {
			img, err := imaging.Open(media.Location)
			if err != nil {
				log.Warn("Non-fatal error getting thumbnail dimensions: " + err.Error())
			} else {
				result.ImageMxc = media.MxcUri()
				result.ImageType = media.ContentType
				result.ImageSize = media.SizeBytes
				result.ImageWidth = img.Bounds().Max.X
				result.ImageHeight = img.Bounds().Max.Y
			}
		}
	}

	dbRecord := &types.CachedUrlPreview{
		Preview:   result,
		SearchUrl: info.urlStr,
		ErrorCode: "",
		FetchedTs: util.NowMillis(),
	}
	err = svc.store.InsertPreview(dbRecord)
	if err != nil {
		log.Warn("Error caching URL preview: " + err.Error())
		// Non-fatal: Just report it and move on. The worst that happens is we re-cache it.
	}

	return &urlPreviewResponse{preview: result}
}

func (h *urlResourceHandler) GeneratePreview(urlStr string, forUserId string, onHost string) chan *urlPreviewResponse {
	resultChan := make(chan *urlPreviewResponse)
	go func() {
		reqId := fmt.Sprintf("preview_%s", urlStr) // don't put the user id or host in the ID string
		result := <-h.resourceHandler.GetResource(reqId, &urlPreviewRequest{
			urlStr:    urlStr,
			forUserId: forUserId,
			onHost:    onHost,
		})
		resultChan <- result.(*urlPreviewResponse)
	}()
	return resultChan
}
