package download_controller

import (
	"errors"
	"io"
	"io/ioutil"
	"mime"
	"strconv"
	"sync"
	"time"

	"github.com/djherbis/stream"
	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/upload_controller"
	"github.com/turt2live/matrix-media-repo/matrix"
	"github.com/turt2live/matrix-media-repo/metrics"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
	"github.com/turt2live/matrix-media-repo/util/resource_handler"
)

type mediaResourceHandler struct {
	resourceHandler *resource_handler.ResourceHandler
}

type downloadRequest struct {
	origin        string
	mediaId       string
	blockForMedia bool
}

type downloadResponse struct {
	err error

	// This is only populated if the request was blocked pending this object
	media *types.Media

	// These properties are populated if `media` is nil
	filename    string
	contentType string
	stream      io.ReadCloser
}

type workerDownloadResponse struct {
	err error

	// This is only populated if the request was blocked pending this object
	media *types.Media

	// These properties are populated if `media` is nil
	filename    string
	contentType string
	stream      *stream.Stream
}

type downloadedMedia struct {
	Contents        io.ReadCloser
	DesiredFilename string
	ContentType     string
	ContentLength   int64
}

var resHandler *mediaResourceHandler
var resHandlerLock = &sync.Once{}
var downloadErrorsCache *cache.Cache
var downloadErrorCacheSingletonLock = &sync.Once{}

func getResourceHandler() *mediaResourceHandler {
	if resHandler == nil {
		resHandlerLock.Do(func() {
			handler, err := resource_handler.New(config.Get().Downloads.NumWorkers, downloadResourceWorkFn)
			if err != nil {
				panic(err)
			}

			resHandler = &mediaResourceHandler{handler}
		})
	}

	return resHandler
}

func (h *mediaResourceHandler) DownloadRemoteMedia(origin string, mediaId string, blockForMedia bool) chan *downloadResponse {
	resultChan := make(chan *downloadResponse)
	go func() {
		reqId := "remote_download:" + origin + "_" + mediaId
		c := h.resourceHandler.GetResource(reqId, &downloadRequest{origin, mediaId, blockForMedia})
		defer close(c)
		result := <-c

		// Translate the response stream into something that is safe to support multiple readers
		resp := result.(*workerDownloadResponse)
		respValue := &downloadResponse{
			err:         resp.err,
			media:       resp.media,
			contentType: resp.contentType,
			filename:    resp.filename,
		}
		if resp.stream != nil {
			s, err := resp.stream.NextReader()
			if err != nil {
				logrus.Error("Unexpected error in processing response for remote media download: ", err)
				respValue = &downloadResponse{err: err}
			} else {
				respValue.stream = s
			}
		}

		resultChan <- respValue
	}()
	return resultChan
}

func downloadResourceWorkFn(request *resource_handler.WorkRequest) interface{} {
	info := request.Metadata.(*downloadRequest)
	ctx := rcontext.Initial().LogWithFields(logrus.Fields{
		"worker_requestId":      request.Id,
		"worker_requestOrigin":  info.origin,
		"worker_requestMediaId": info.mediaId,
		"worker_blockForMedia":  info.blockForMedia,
	})
	ctx.Log.Info("Downloading remote media")

	downloaded, err := DownloadRemoteMediaDirect(info.origin, info.mediaId, ctx)
	if err != nil {
		return &workerDownloadResponse{err: err}
	}

	persistFile := func(fileStream io.ReadCloser) *workerDownloadResponse {
		defer cleanup.DumpAndCloseStream(fileStream)
		userId := upload_controller.NoApplicableUploadUser
		media, err := upload_controller.StoreDirect(nil, fileStream, downloaded.ContentLength, downloaded.ContentType, downloaded.DesiredFilename, userId, info.origin, info.mediaId, common.KindRemoteMedia, ctx, true)
		if err != nil {
			ctx.Log.Error("Error persisting file: ", err)
			return &workerDownloadResponse{err: err}
		}

		ctx.Log.Info("Remote media persisted under datastore ", media.DatastoreId, " at ", media.Location)
		return &workerDownloadResponse{media: media, contentType: media.ContentType}
	}

	if info.blockForMedia {
		ctx.Log.Warn("Not streaming remote media download request due to request for a block")
		return persistFile(downloaded.Contents)
	}

	ctx.Log.Info("Streaming remote media to filesystem and requesting party at the same time")

	reader, writer := io.Pipe()
	tr := io.TeeReader(downloaded.Contents, writer)

	go persistFile(ioutil.NopCloser(tr))

	ms := stream.NewMemStream()
	defer ms.Close()
	io.Copy(ms, reader)

	return &workerDownloadResponse{
		err:         nil,
		contentType: downloaded.ContentType,
		filename:    downloaded.DesiredFilename,
		stream:      ms,
	}
}

func DownloadRemoteMediaDirect(server string, mediaId string, ctx rcontext.RequestContext) (*downloadedMedia, error) {
	if downloadErrorsCache == nil {
		downloadErrorCacheSingletonLock.Do(func() {
			cacheTime := time.Duration(ctx.Config.Downloads.FailureCacheMinutes) * time.Minute
			downloadErrorsCache = cache.New(cacheTime, cacheTime*2)
		})
	}

	cacheKey := server + "/" + mediaId
	item, found := downloadErrorsCache.Get(cacheKey)
	if found {
		ctx.Log.Warn("Returning cached error for remote media download failure")
		return nil, item.(error)
	}

	baseUrl, realHost, err := matrix.GetServerApiUrl(server)
	if err != nil {
		downloadErrorsCache.Set(cacheKey, err, cache.DefaultExpiration)
		return nil, err
	}

	downloadUrl := baseUrl + "/_matrix/media/r0/download/" + server + "/" + mediaId + "?allow_remote=false"
	resp, err := matrix.FederatedGet(downloadUrl, realHost, ctx)
	if err != nil {
		downloadErrorsCache.Set(cacheKey, err, cache.DefaultExpiration)
		return nil, err
	}

	if resp.StatusCode == 404 {
		ctx.Log.Info("Remote media not found")

		err = common.ErrMediaNotFound
		downloadErrorsCache.Set(cacheKey, err, cache.DefaultExpiration)
		return nil, err
	} else if resp.StatusCode != 200 {
		ctx.Log.Info("Unknown error fetching remote media; received status code " + strconv.Itoa(resp.StatusCode))

		err = errors.New("could not fetch remote media")
		downloadErrorsCache.Set(cacheKey, err, cache.DefaultExpiration)
		return nil, err
	}

	var contentLength int64 = 0
	if resp.Header.Get("Content-Length") != "" {
		contentLength, err = strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
		if err != nil {
			return nil, err
		}
	} else {
		ctx.Log.Warn("Missing Content-Length header on response - continuing anyway")
	}

	if contentLength > 0 && ctx.Config.Downloads.MaxSizeBytes > 0 && contentLength > ctx.Config.Downloads.MaxSizeBytes {
		ctx.Log.Warn("Attempted to download media that was too large")

		err = common.ErrMediaTooLarge
		downloadErrorsCache.Set(cacheKey, err, cache.DefaultExpiration)
		return nil, err
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		ctx.Log.Warn("Remote media has no content type; Assuming application/octet-stream")
		contentType = "application/octet-stream" // binary
	}

	request := &downloadedMedia{
		ContentType:   contentType,
		Contents:      resp.Body,
		ContentLength: contentLength,
		// DesiredFilename (calculated below)
	}

	_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition"))
	if err == nil && params["filename"] != "" {
		request.DesiredFilename = params["filename"]
	}

	ctx.Log.Info("Persisting downloaded media")
	metrics.MediaDownloaded.With(prometheus.Labels{"origin": server}).Inc()
	return request, nil
}
