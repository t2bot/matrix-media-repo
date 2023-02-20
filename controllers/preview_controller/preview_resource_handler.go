package preview_controller

import (
	"fmt"
	"sync"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/util/stream_util"

	"github.com/disintegration/imaging"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/preview_controller/preview_types"
	"github.com/turt2live/matrix-media-repo/controllers/preview_controller/previewers"
	"github.com/turt2live/matrix-media-repo/controllers/upload_controller"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/resource_handler"
)

type urlResourceHandler struct {
	resourceHandler *resource_handler.ResourceHandler
}

type urlPreviewRequest struct {
	urlPayload     *preview_types.UrlPayload
	forUserId      string
	onHost         string
	languageHeader string
	allowOEmbed    bool
}

type urlPreviewResponse struct {
	preview *types.UrlPreview
	err     error
}

var resHandlerInstance *urlResourceHandler
var resHandlerSingletonLock = &sync.Once{}

func getResourceHandler() *urlResourceHandler {
	if resHandlerInstance == nil {
		resHandlerSingletonLock.Do(func() {
			handler, err := resource_handler.New(config.Get().UrlPreviews.NumWorkers, func(r *resource_handler.WorkRequest) interface{} {
				return urlPreviewWorkFn(r)
			})
			if err != nil {
				sentry.CaptureException(err)
				panic(err)
			}

			resHandlerInstance = &urlResourceHandler{handler}
		})
	}

	return resHandlerInstance
}

func urlPreviewWorkFn(request *resource_handler.WorkRequest) (resp *urlPreviewResponse) {
	info := request.Metadata.(*urlPreviewRequest)
	ctx := rcontext.Initial().LogWithFields(logrus.Fields{
		"worker_requestId": request.Id,
		"worker_url":       info.urlPayload.UrlString,
	})

	resp = &urlPreviewResponse{}
	defer func() {
		if err := recover(); err != nil {
			ctx.Log.Error("Caught panic: ", err)
			sentry.CurrentHub().Recover(err)
			resp.preview = nil
			resp.err = util.PanicToError(err)
		}
	}()

	ctx.Log.Info("Processing url preview request")

	db := storage.GetDatabase().GetUrlStore(ctx)

	var preview preview_types.PreviewResult
	err := preview_types.ErrPreviewUnsupported

	// Try oEmbed first
	if info.allowOEmbed {
		ctx = ctx.LogWithFields(logrus.Fields{"worker_previewer": "oEmbed"})
		ctx.Log.Info("Trying oEmbed previewer")
		preview, err = previewers.GenerateOEmbedPreview(info.urlPayload, info.languageHeader, ctx)
	}

	// Then try OpenGraph
	if err == preview_types.ErrPreviewUnsupported {
		ctx = ctx.LogWithFields(logrus.Fields{"worker_previewer": "OpenGraph"})
		ctx.Log.Info("oEmbed preview for this URL is unsupported or disabled - treating it as a OpenGraph")
		preview, err = previewers.GenerateOpenGraphPreview(info.urlPayload, info.languageHeader, ctx)
	}

	// Finally try scraping
	if err == preview_types.ErrPreviewUnsupported {
		ctx = ctx.LogWithFields(logrus.Fields{"worker_previewer": "File"})
		ctx.Log.Info("OpenGraph preview for this URL is unsupported - treating it as a file")
		preview, err = previewers.GenerateCalculatedPreview(info.urlPayload, info.languageHeader, ctx)
	}

	if err != nil {
		// Transparently convert "unsupported" to "not found" for processing
		if err == preview_types.ErrPreviewUnsupported {
			err = common.ErrMediaNotFound
		}

		if err == common.ErrMediaNotFound {
			db.InsertPreviewError(info.urlPayload.UrlString, common.ErrCodeNotFound)
		} else {
			db.InsertPreviewError(info.urlPayload.UrlString, common.ErrCodeUnknown)
		}
		resp.err = err
		return resp
	}

	result := &types.UrlPreview{
		Url:            preview.Url,
		SiteName:       preview.SiteName,
		Type:           preview.Type,
		Description:    preview.Description,
		Title:          preview.Title,
		LanguageHeader: info.languageHeader,
	}

	// Store the thumbnail, if there is one
	if preview.Image != nil && !upload_controller.IsRequestTooLarge(preview.Image.ContentLength, preview.Image.ContentLengthHeader, ctx) {
		contentLength := upload_controller.EstimateContentLength(preview.Image.ContentLength, preview.Image.ContentLengthHeader)

		// UploadMedia will close the read stream for the thumbnail and dedupe the image
		media, err := upload_controller.UploadMedia(preview.Image.Data, contentLength, preview.Image.ContentType, preview.Image.Filename, info.forUserId, info.onHost, ctx)
		if err != nil {
			ctx.Log.Warn("Non-fatal error storing preview thumbnail: " + err.Error())
			sentry.CaptureException(err)
		} else {
			mediaStream, err := datastore.DownloadStream(ctx, media.DatastoreId, media.Location)
			if err != nil {
				ctx.Log.Warn("Non-fatal error streaming datastore file: " + err.Error())
				sentry.CaptureException(err)
			} else {
				defer stream_util.DumpAndCloseStream(mediaStream)
				img, err := imaging.Decode(mediaStream)
				if err != nil {
					ctx.Log.Warn("Non-fatal error getting thumbnail dimensions: " + err.Error())
					sentry.CaptureException(err)
				} else {
					result.ImageMxc = media.MxcUri()
					result.ImageType = media.ContentType
					result.ImageSize = media.SizeBytes
					result.ImageWidth = img.Bounds().Max.X
					result.ImageHeight = img.Bounds().Max.Y
				}
			}
		}
	}

	dbRecord := &types.CachedUrlPreview{
		Preview:   result,
		SearchUrl: info.urlPayload.UrlString,
		ErrorCode: "",
		FetchedTs: util.NowMillis(),
	}
	err = db.InsertPreview(dbRecord)
	if err != nil {
		ctx.Log.Warn("Error caching URL preview: " + err.Error())
		sentry.CaptureException(err)
		// Non-fatal: Just report it and move on. The worst that happens is we re-cache it.
	}

	resp.preview = result
	return resp
}

func (h *urlResourceHandler) GeneratePreview(urlPayload *preview_types.UrlPayload, forUserId string, onHost string, languageHeader string, allowOEmbed bool) chan *urlPreviewResponse {
	resultChan := make(chan *urlPreviewResponse)
	go func() {
		reqId := fmt.Sprintf("preview_%s", urlPayload.UrlString) // don't put the user id or host in the ID string
		c := h.resourceHandler.GetResource(reqId, &urlPreviewRequest{
			urlPayload:     urlPayload,
			forUserId:      forUserId,
			onHost:         onHost,
			languageHeader: languageHeader,
			allowOEmbed:    allowOEmbed,
		})
		defer close(c)
		result := <-c
		resultChan <- result.(*urlPreviewResponse)
	}()
	return resultChan
}
