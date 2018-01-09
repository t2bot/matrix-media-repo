package services

import (
	"context"
	"database/sql"
	"errors"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/services/handlers"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/stores"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/errs"
)

type ThumbnailService struct {
	store *stores.ThumbnailStore
	ctx   context.Context
	log   *logrus.Entry
}

func NewThumbnailService(ctx context.Context, log *logrus.Entry) (*ThumbnailService) {
	store := storage.GetDatabase().GetThumbnailStore(ctx, log)
	return &ThumbnailService{store, ctx, log}
}

func (s *ThumbnailService) GetThumbnail(media *types.Media, width int, height int, method string) (*types.Thumbnail, error) {
	if width <= 0 {
		return nil, errors.New("width must be positive")
	}
	if height <= 0 {
		return nil, errors.New("height must be positive")
	}
	if method != "crop" && method != "scale" {
		return nil, errors.New("method must be crop or scale")
	}

	targetWidth := width
	targetHeight := height
	foundFirst := false

	for i := 0; i < len(config.Get().Thumbnails.Sizes); i++ {
		size := config.Get().Thumbnails.Sizes[i]
		lastSize := i == len(config.Get().Thumbnails.Sizes)-1

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

	s.log = s.log.WithFields(logrus.Fields{
		"targetWidth":  targetWidth,
		"targetHeight": targetHeight,
	})
	s.log.Info("Looking up thumbnail")

	thumb, err := s.store.Get(media.Origin, media.MediaId, targetWidth, targetHeight, method)
	if err != nil && err != sql.ErrNoRows {
		s.log.Error("Unexpected error processing thumbnail lookup: " + err.Error())
		return thumb, err
	}
	if err != sql.ErrNoRows {
		s.log.Info("Found existing thumbnail")
		return thumb, nil
	}

	if media.SizeBytes > config.Get().Thumbnails.MaxSourceBytes {
		s.log.Warn("Media too large to thumbnail")
		return thumb, errs.ErrMediaTooLarge
	}

	s.log.Info("Generating new thumbnail")
	thumbnailer := handlers.NewThumbnailer(s.ctx, s.log)

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
		CreationTs:  util.NowMillis(),
		ContentType: generated.ContentType,
		Location:    generated.DiskLocation,
		SizeBytes:   generated.SizeBytes,
	}

	err = s.store.Insert(newThumb)
	if err != nil {
		s.log.Error("Unexpected error caching thumbnail: " + err.Error())
		return newThumb, err
	}

	return newThumb, nil
}
