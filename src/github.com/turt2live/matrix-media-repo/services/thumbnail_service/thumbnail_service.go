package thumbnail_service

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/stores"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/errs"
)

// These are the content types that we can actually thumbnail
var supportedThumbnailTypes = []string{"image/jpeg", "image/jpg", "image/png", "image/gif", "image/svg+xml"}

// Of the supportedThumbnailTypes, these are the 'animated' types
var animatedTypes = []string{"image/gif"}

type thumbnailService struct {
	store *stores.ThumbnailStore
	ctx   context.Context
	log   *logrus.Entry
}

func New(ctx context.Context, log *logrus.Entry) (*thumbnailService) {
	store := storage.GetDatabase().GetThumbnailStore(ctx, log)
	return &thumbnailService{store, ctx, log}
}

func (s *thumbnailService) GetThumbnailDirect(media *types.Media, width int, height int, method string, animated bool) (*types.Thumbnail, error) {
	return s.store.Get(media.Origin, media.MediaId, width, height, method, animated)
}

func (s *thumbnailService) GenerateThumbnail(media *types.Media, width int, height int, method string, animated bool) (*types.Thumbnail, error) {
	if !util.ArrayContains(supportedThumbnailTypes, media.ContentType) {
		s.log.Warn("Cannot generate thumbnail for " + media.ContentType + " because it is not supported")
		return nil, errors.New("cannot generate thumbnail for this media's content type")
	}

	if !util.ArrayContains(config.Get().Thumbnails.Types, media.ContentType) {
		s.log.Warn("Cannot generate thumbnail for " + media.ContentType + " because it is not listed in the config")
		return nil, errors.New("cannot generate thumbnail for this media's content type")
	}

	if animated && config.Get().Thumbnails.MaxAnimateSizeBytes > 0 && config.Get().Thumbnails.MaxAnimateSizeBytes < media.SizeBytes {
		s.log.Warn("Attempted to animate a media record that is too large. Assuming animated=false")
		animated = false
	}

	forceThumbnail := false
	if animated && !util.ArrayContains(animatedTypes, media.ContentType) {
		s.log.Warn("Cannot animate a non-animated file. Assuming animated=false")
		animated = false
	}
	if !animated && util.ArrayContains(animatedTypes, media.ContentType) {
		// We have to force a thumbnail otherwise we'll return a non-animated file
		forceThumbnail = true
	}

	if media.SizeBytes > config.Get().Thumbnails.MaxSourceBytes {
		s.log.Warn("Media too large to thumbnail")
		return nil, errs.ErrMediaTooLarge
	}

	s.log.Info("Generating new thumbnail")

	result := <-getResourceHandler().GenerateThumbnail(media, width, height, method, animated, forceThumbnail)
	return result.thumbnail, result.err
}
