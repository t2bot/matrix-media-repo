package services

import (
	"database/sql"
	"errors"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/rcontext"
	"github.com/turt2live/matrix-media-repo/services/handlers"
	"github.com/turt2live/matrix-media-repo/storage/stores"
	"github.com/turt2live/matrix-media-repo/types"
)

type ThumbnailService struct {
	store *stores.ThumbnailStore
	i     rcontext.RequestInfo
}

func CreateThumbnailService(i rcontext.RequestInfo) (*ThumbnailService) {
	return &ThumbnailService{i.Db.GetThumbnailStore(i.Context, i.Log), i}
}

func (s *ThumbnailService) GetThumbnail(media types.Media, width int, height int, method string) (types.Thumbnail, error) {
	if width <= 0 {
		return types.Thumbnail{}, errors.New("width must be positive")
	}
	if height <= 0 {
		return types.Thumbnail{}, errors.New("height must be positive")
	}
	if method != "crop" && method != "scale" {
		return types.Thumbnail{}, errors.New("method must be crop or scale")
	}

	targetWidth := width
	targetHeight := height
	foundFirst := false

	for i := 0; i < len(s.i.Config.Thumbnails.Sizes); i++ {
		size := s.i.Config.Thumbnails.Sizes[i]
		lastSize := i == len(s.i.Config.Thumbnails.Sizes)-1

		if width == size.Width && height == size.Height {
			targetWidth = width
			targetHeight = height
			break
		}

		if (size.Width < width || size.Height < height) && !lastSize {
			continue // too small
		}

		diffWidth := size.Width - width
		diffHeight := size.Height - height
		currDiffWidth := targetWidth - width
		currDiffHeight := targetHeight - height

		diff := diffWidth + diffHeight
		currDiff := currDiffWidth + currDiffHeight

		if !foundFirst || diff < currDiff || lastSize {
			foundFirst = true
			targetWidth = size.Width
			targetHeight = size.Height
		}
	}

	s.i.Log = s.i.Log.WithFields(logrus.Fields{
		"targetWidth":  targetWidth,
		"targetHeight": targetHeight,
	})
	s.i.Log.Info("Looking up thumbnail")

	thumb, err := s.store.Get(media.Origin, media.MediaId, targetWidth, targetHeight, method)
	if err != nil && err != sql.ErrNoRows {
		s.i.Log.Error("Unexpected error processing thumbnail lookup: " + err.Error())
		return thumb, err
	}
	if err != sql.ErrNoRows {
		s.i.Log.Info("Found existing thumbnail")
		return thumb, nil
	}

	if media.SizeBytes > s.i.Config.Thumbnails.MaxSourceBytes {
		s.i.Log.Warn("Media too large to thumbnail")
		return thumb, errors.New("cannot thumbnail, image too large")
	}

	s.i.Log.Info("Generating new thumbnail")
	thumbnailer := &handlers.Thumbnailer{
		Info:           s.i,
		ThumbnailStore: *s.store,
	}

	generated, err := thumbnailer.GenerateThumbnail(media, targetWidth, targetHeight, method)
	if err != nil {
		return thumb, nil
	}

	newThumb := &types.Thumbnail{
		Origin:      media.Origin,
		MediaId:     media.MediaId,
		Width:       targetWidth,
		Height:      targetHeight,
		Method:      method,
		CreationTs:  time.Now().UnixNano() / 1000000,
		ContentType: generated.ContentType,
		Location:    generated.DiskLocation,
		SizeBytes:   generated.SizeBytes,
	}

	err = s.store.Insert(newThumb)
	if err != nil {
		s.i.Log.Error("Unexpected error caching thumbnail: " + err.Error())
		return *newThumb, err
	}

	return *newThumb, nil
}
