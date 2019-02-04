package download_controller

import (
	"context"
	"errors"
	"io"
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
	"github.com/turt2live/matrix-media-repo/controllers/upload_controller"
	"github.com/turt2live/matrix-media-repo/matrix"
	"github.com/turt2live/matrix-media-repo/metrics"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
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
}

var resHandler *mediaResourceHandler
var resHandlerLock = &sync.Once{}
var downloadErrorsCache *cache.Cache
var downloadErrorCacheSingletonLock = &sync.Once{}

func getResourceHandler() (*mediaResourceHandler) {
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
		result := <-h.resourceHandler.GetResource(reqId, &downloadRequest{origin, mediaId, blockForMedia})

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
	log := logrus.WithFields(logrus.Fields{
		"worker_requestId":      request.Id,
		"worker_requestOrigin":  info.origin,
		"worker_requestMediaId": info.mediaId,
		"worker_blockForMedia":  info.blockForMedia,
	})
	log.Info("Downloading remote media")

	ctx := context.TODO() // TODO: Should we use a real context?

	downloaded, err := DownloadRemoteMediaDirect(info.origin, info.mediaId, log)
	if err != nil {
		return &workerDownloadResponse{err: err}
	}

	log.Info("Checking to ensure the reported content type is allowed...")
	if downloaded.ContentType != "" && !upload_controller.IsAllowed(downloaded.ContentType, downloaded.ContentType, upload_controller.NoApplicableUploadUser, log) {
		log.Error("Remote media failed the preliminary IsAllowed check based on content type (reported as " + downloaded.ContentType + ")")
		return &workerDownloadResponse{err: common.ErrMediaNotAllowed}
	}

	persistFile := func(fileStream io.ReadCloser) (*workerDownloadResponse) {
		defer fileStream.Close()

		userId := upload_controller.NoApplicableUploadUser
		media, err := upload_controller.StoreDirect(fileStream, downloaded.ContentType, downloaded.DesiredFilename, userId, info.origin, info.mediaId, ctx, log)
		if err != nil {
			log.Error("Error persisting file: ", err)
			return &workerDownloadResponse{err: err}
		}

		log.Info("Remote media persisted under datastore ", media.DatastoreId, " at ", media.Location)
		return &workerDownloadResponse{media: media}
	}

	if info.blockForMedia {
		log.Warn("Not streaming remote media download request due to request for a block")
		return persistFile(downloaded.Contents)
	}

	log.Info("Streaming remote media to filesystem and requesting party at the same time")
	readers := util.CloneReader(downloaded.Contents, 2)
	returnedReader := readers[0]
	persistReader := readers[1]

	go persistFile(persistReader)

	ms := stream.NewMemStream()
	go func() {
		defer ms.Close()
		io.Copy(ms, returnedReader)
	}()

	return &workerDownloadResponse{
		err:         nil,
		contentType: downloaded.ContentType,
		filename:    downloaded.DesiredFilename,
		stream:      ms,
	}
}

func DownloadRemoteMediaDirect(server string, mediaId string, log *logrus.Entry) (*downloadedMedia, error) {
	if downloadErrorsCache == nil {
		downloadErrorCacheSingletonLock.Do(func() {
			cacheTime := time.Duration(config.Get().Downloads.FailureCacheMinutes) * time.Minute
			downloadErrorsCache = cache.New(cacheTime, cacheTime*2)
		})
	}

	cacheKey := server + "/" + mediaId
	item, found := downloadErrorsCache.Get(cacheKey)
	if found {
		log.Warn("Returning cached error for remote media download failure")
		return nil, item.(error)
	}

	baseUrl, realHost, err := matrix.GetServerApiUrl(server)
	if err != nil {
		downloadErrorsCache.Set(cacheKey, err, cache.DefaultExpiration)
		return nil, err
	}

	downloadUrl := baseUrl + "/_matrix/media/v1/download/" + server + "/" + mediaId + "?allow_remote=false"
	resp, err := matrix.FederatedGet(downloadUrl, realHost)
	if err != nil {
		downloadErrorsCache.Set(cacheKey, err, cache.DefaultExpiration)
		return nil, err
	}

	if resp.StatusCode == 404 {
		log.Info("Remote media not found")

		err = common.ErrMediaNotFound
		downloadErrorsCache.Set(cacheKey, err, cache.DefaultExpiration)
		return nil, err
	} else if resp.StatusCode != 200 {
		log.Info("Unknown error fetching remote media; received status code " + strconv.Itoa(resp.StatusCode))

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
		log.Warn("Missing Content-Length header on response - continuing anyway")
	}

	if contentLength > 0 && config.Get().Downloads.MaxSizeBytes > 0 && contentLength > config.Get().Downloads.MaxSizeBytes {
		log.Warn("Attempted to download media that was too large")

		err = common.ErrMediaTooLarge
		downloadErrorsCache.Set(cacheKey, err, cache.DefaultExpiration)
		return nil, err
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		log.Warn("Remote media has no content type; Assuming application/octet-stream")
		contentType = "application/octet-stream" // binary
	}

	request := &downloadedMedia{
		ContentType: contentType,
		Contents:    resp.Body,
		// DesiredFilename (calculated below)
	}

	_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition"))
	if err == nil && params["filename"] != "" {
		request.DesiredFilename = params["filename"]
	}

	log.Info("Persisting downloaded media")
	metrics.MediaDownloaded.With(prometheus.Labels{"origin": server}).Inc()
	return request, nil
}
