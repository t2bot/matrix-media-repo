package services

import (
	"database/sql"

	"github.com/turt2live/matrix-media-repo/rcontext"
	"github.com/turt2live/matrix-media-repo/services/handlers"
	"github.com/turt2live/matrix-media-repo/storage/stores"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type MediaService struct {
	store *stores.MediaStore
	i     rcontext.RequestInfo
}

func CreateMediaService(i rcontext.RequestInfo) (*MediaService) {
	return &MediaService{i.Db.GetMediaStore(i.Context, i.Log), i}
}

func (s *MediaService) GetMedia(server string, mediaId string) (types.Media, error) {
	s.i.Log.Info("Looking up media")
	media, err := s.store.Get(server, mediaId)
	if err != nil {
		if err == sql.ErrNoRows {
			if util.IsServerOurs(server, s.i.Config) {
				s.i.Log.Warn("Media not found")
				return media, util.ErrMediaNotFound
			}
		}

		s.i.Log.Info("Attempting to download remote media")
		downloader := &handlers.RemoteMediaDownloader{
			Info:       s.i,
			MediaStore: *s.store,
		}

		downloaded, err := downloader.Download(server, mediaId)
		if err != nil {
			return types.Media{}, err
		}

		request := &handlers.MediaUploadRequest{
			DesiredFilename: downloaded.DesiredFilename,
			ContentType:     downloaded.ContentType,
			Contents:        downloaded.Contents,
			Host:            server,
			UploadedBy:      "",
		}
		return request.StoreMedia(s.i)
	}

	return media, nil
}
