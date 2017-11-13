package media_handler

import (
	"context"
	"database/sql"
	"errors"
	"mime"
	"strconv"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

var ErrMediaNotFound = errors.New("media not found")
var ErrMediaTooLarge = errors.New("media too large")

func FindMedia(ctx context.Context, server string, mediaId string, c config.MediaRepoConfig, db storage.Database) (types.Media, error) {
	media, err := db.GetMedia(ctx, server, mediaId)
	if err != nil {
		if err == sql.ErrNoRows {
			if util.IsServerOurs(server, c) {
				return media, ErrMediaNotFound
			}

			media, err = DownloadMedia(ctx, server, mediaId, c, db)
			return media, err
		}
		return media, err
	}

	return media, nil
}

func DownloadMedia(ctx context.Context, server string, mediaId string, c config.MediaRepoConfig, db storage.Database) (types.Media, error) {
	request := &MediaUploadRequest{
		UploadedBy: "",
		Host:       server,
		//ContentType:
		//DesiredFilename:
		//Contents:
	}

	mtxClient := gomatrixserverlib.NewClient()
	mtxServer := gomatrixserverlib.ServerName(server)
	resp, err := mtxClient.CreateMediaDownloadRequest(ctx, mtxServer, mediaId)
	if err != nil {
		return types.Media{}, err
	}

	if resp.StatusCode == 404 {
		return types.Media{}, ErrMediaNotFound
	} else if resp.StatusCode != 200 {
		return types.Media{}, errors.New("could not fetch remote media")
	}

	defer resp.Body.Close()
	contentLength, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return types.Media{}, err
	}
	if c.Downloads.MaxSizeBytes > 0 && contentLength > c.Downloads.MaxSizeBytes {
		return types.Media{}, ErrMediaTooLarge
	}

	request.ContentType = resp.Header.Get("Content-Type")
	request.Contents = resp.Body

	_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition"))
	if err == nil && params["filename"] != "" {
		request.DesiredFilename = params["filename"]
	}

	return request.StoreMedia(ctx, c, db)
}
