package download_controller

import (
	"context"
	"errors"
	"io"
	"mime"
	"strconv"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/matrix"
)

type downloadedMedia struct {
	Contents        io.ReadCloser
	DesiredFilename string
	ContentType     string
}

type remoteMediaDownloader struct {
	ctx context.Context
	log *logrus.Entry
}

var downloadErrorsCache *cache.Cache
var downloadErrorCacheSingletonLock = &sync.Once{}

func newRemoteMediaDownloader(ctx context.Context, log *logrus.Entry) *remoteMediaDownloader {
	return &remoteMediaDownloader{ctx, log}
}

func (r *remoteMediaDownloader) Download(server string, mediaId string) (*downloadedMedia, error) {
	if downloadErrorsCache == nil {
		downloadErrorCacheSingletonLock.Do(func() {
			cacheTime := time.Duration(config.Get().Downloads.FailureCacheMinutes) * time.Minute
			downloadErrorsCache = cache.New(cacheTime, cacheTime*2)
		})
	}

	cacheKey := server + "/" + mediaId
	item, found := downloadErrorsCache.Get(cacheKey)
	if found {
		r.log.Warn("Returning cached error for remote media download failure")
		return nil, item.(error)
	}

	baseUrl, err := matrix.GetServerApiUrl(server)
	if err != nil {
		downloadErrorsCache.Set(cacheKey, err, cache.DefaultExpiration)
		return nil, err
	}

	downloadUrl := baseUrl + "/_matrix/media/v1/download/" + server + "/" + mediaId + "?allow_remote=false"
	resp, err := matrix.FederatedGet(downloadUrl, server)
	if err != nil {
		downloadErrorsCache.Set(cacheKey, err, cache.DefaultExpiration)
		return nil, err
	}

	if resp.StatusCode == 404 {
		r.log.Info("Remote media not found")

		err = common.ErrMediaNotFound
		downloadErrorsCache.Set(cacheKey, err, cache.DefaultExpiration)
		return nil, err
	} else if resp.StatusCode != 200 {
		r.log.Info("Unknown error fetching remote media; received status code " + strconv.Itoa(resp.StatusCode))

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
		r.log.Warn("Missing Content-Length header on response - continuing anyway")
	}

	if contentLength > 0 && config.Get().Downloads.MaxSizeBytes > 0 && contentLength > config.Get().Downloads.MaxSizeBytes {
		r.log.Warn("Attempted to download media that was too large")

		err = common.ErrMediaTooLarge
		downloadErrorsCache.Set(cacheKey, err, cache.DefaultExpiration)
		return nil, err
	}

	request := &downloadedMedia{
		ContentType: resp.Header.Get("Content-Type"),
		Contents:    resp.Body,
		// DesiredFilename (calculated below)
	}

	_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition"))
	if err == nil && params["filename"] != "" {
		request.DesiredFilename = params["filename"]
	}

	r.log.Info("Persisting downloaded media")
	return request, nil
}
