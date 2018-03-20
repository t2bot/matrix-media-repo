package thumbnail_service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"math"

	"github.com/disintegration/imaging"
	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/stores"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/errs"
	"golang.org/x/image/font/gofont/gosmallcaps"
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
		return nil, errs.ErrMediaNotAnimated
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

func (s *thumbnailService) GenerateQuarantineThumbnail(server string, mediaId string, width int, height int) (image.Image, error) {
	var centerImage image.Image
	var err error
	if config.Get().Quarantine.ThumbnailPath != "" {
		centerImage, err = imaging.Open(config.Get().Quarantine.ThumbnailPath)
	} else {
		centerImage, err = s.generateDefaultQuarantineThumbnail()
	}
	if err != nil {
		return nil, err
	}

	c := gg.NewContext(width, height)

	centerImage = imaging.Fit(centerImage, width, height, imaging.Lanczos)

	c.DrawImageAnchored(centerImage, width/2, height/2, 0.5, 0.5)

	buf := &bytes.Buffer{}
	c.EncodePNG(buf)

	return imaging.Decode(buf)
}

func (s *thumbnailService) generateDefaultQuarantineThumbnail() (image.Image, error) {
	c := gg.NewContext(700, 700)
	c.Clear()

	red := color.RGBA{R: 190, G: 26, B: 25, A: 255}
	x := 350.0
	y := 300.0
	r := 256.0
	w := 55.0
	p := 64.0
	m := "media not allowed"

	c.SetColor(red)
	c.DrawCircle(x, y, r)
	c.Fill()

	c.SetColor(color.White)
	c.DrawCircle(x, y, r-w)
	c.Fill()

	lr := r - (w / 2)
	sx := x + (lr * math.Cos(gg.Radians(225.0)))
	sy := y + (lr * math.Sin(gg.Radians(225.0)))
	ex := x + (lr * math.Cos(gg.Radians(45.0)))
	ey := y + (lr * math.Sin(gg.Radians(45.0)))
	c.SetLineCap(gg.LineCapButt)
	c.SetLineWidth(w)
	c.SetColor(red)
	c.DrawLine(sx, sy, ex, ey)
	c.Stroke()

	f, err := truetype.Parse(gosmallcaps.TTF)
	if err != nil {
		panic(err)
	}

	c.SetColor(color.Black)
	c.SetFontFace(truetype.NewFace(f, &truetype.Options{Size: 64}))
	c.DrawStringAnchored(m, x, y+r+p, 0.5, 0.5)

	buf := &bytes.Buffer{}
	c.EncodePNG(buf)

	return imaging.Decode(buf)
}
