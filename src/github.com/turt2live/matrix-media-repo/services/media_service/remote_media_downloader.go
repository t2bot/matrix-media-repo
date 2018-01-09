package media_service

import (
	"context"
	"errors"
	"io"
	"mime"
	"strconv"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/util/errs"
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

func newRemoteMediaDownloader(ctx context.Context, log *logrus.Entry) *remoteMediaDownloader {
	return &remoteMediaDownloader{ctx, log}
}

func (r *remoteMediaDownloader) Download(server string, mediaId string) (*downloadedMedia, error) {
	mtxClient := gomatrixserverlib.NewClient()
	mtxServer := gomatrixserverlib.ServerName(server)
	resp, err := mtxClient.CreateMediaDownloadRequest(r.ctx, mtxServer, mediaId)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 404 {
		r.log.Info("Remote media not found")
		return nil, errs.ErrMediaNotFound
	} else if resp.StatusCode != 200 {
		r.log.Info("Unknown error fetching remote media; received status code " + strconv.Itoa(resp.StatusCode))
		return nil, errors.New("could not fetch remote media")
	}

	contentLength, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return nil, err
	}
	if config.Get().Downloads.MaxSizeBytes > 0 && contentLength > config.Get().Downloads.MaxSizeBytes {
		r.log.Warn("Attempted to download media that was too large")
		return nil, errs.ErrMediaTooLarge
	}

	request := &downloadedMedia{
		ContentType: resp.Header.Get("Content-Type"),
		Contents:    resp.Body,
		//DesiredFilename (calculated below)
	}

	_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition"))
	if err == nil && params["filename"] != "" {
		request.DesiredFilename = params["filename"]
	}

	r.log.Info("Persisting downloaded media")
	return request, nil
}
