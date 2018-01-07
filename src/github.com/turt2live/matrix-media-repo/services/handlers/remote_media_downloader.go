package handlers

import (
	"errors"
	"io"
	"mime"
	"strconv"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/turt2live/matrix-media-repo/rcontext"
	"github.com/turt2live/matrix-media-repo/storage/stores"
	"github.com/turt2live/matrix-media-repo/util"
)

type DownloadedMedia struct {
	Contents        io.Reader
	DesiredFilename string
	ContentType     string
}

type RemoteMediaDownloader struct {
	MediaStore stores.MediaStore
	Info       rcontext.RequestInfo
}

func (r *RemoteMediaDownloader) Download(server string, mediaId string) (DownloadedMedia, error) {
	mtxClient := gomatrixserverlib.NewClient()
	mtxServer := gomatrixserverlib.ServerName(server)
	resp, err := mtxClient.CreateMediaDownloadRequest(r.Info.Context, mtxServer, mediaId)
	if err != nil {
		return DownloadedMedia{}, err
	}

	if resp.StatusCode == 404 {
		r.Info.Log.Info("Remote media not found")
		return DownloadedMedia{}, util.ErrMediaNotFound
	} else if resp.StatusCode != 200 {
		r.Info.Log.Info("Unknown error fetching remote media; received status code " + strconv.Itoa(resp.StatusCode))
		return DownloadedMedia{}, errors.New("could not fetch remote media")
	}

	contentLength, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return DownloadedMedia{}, err
	}
	if r.Info.Config.Downloads.MaxSizeBytes > 0 && contentLength > r.Info.Config.Downloads.MaxSizeBytes {
		r.Info.Log.Warn("Attempted to download media that was too large")
		return DownloadedMedia{}, util.ErrMediaTooLarge
	}

	request := &DownloadedMedia{
		ContentType: resp.Header.Get("Content-Type"),
		Contents:    resp.Body,
		//DesiredFilename (calculated below)
	}

	_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition"))
	if err == nil && params["filename"] != "" {
		request.DesiredFilename = params["filename"]
	}

	r.Info.Log.Info("Persisting downloaded media")
	return *request, nil
}
