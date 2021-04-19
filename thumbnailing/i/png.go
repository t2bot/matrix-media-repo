package i

import (
	"bytes"
	"errors"
	"image"
	_ "image/png"
	"io/ioutil"

	"github.com/disintegration/imaging"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/thumbnailing/m"
	"github.com/turt2live/matrix-media-repo/thumbnailing/u"
)

type pngGenerator struct {
}

func (d pngGenerator) supportedContentTypes() []string {
	return []string{"image/png"}
}

func (d pngGenerator) supportsAnimation() bool {
	return false
}

func (d pngGenerator) matches(img []byte, contentType string) bool {
	return contentType == "image/png"
}

func (d pngGenerator) GetOriginDimensions(b []byte, contentType string, ctx rcontext.RequestContext) (bool, int, int, error) {
	i, _, err := image.DecodeConfig(bytes.NewBuffer(b))
	if err != nil {
		return false, 0, 0, err
	}
	return true, i.Width, i.Height, nil
}

func (d pngGenerator) GenerateThumbnail(b []byte, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	src, err := imaging.Decode(bytes.NewBuffer(b))
	if err != nil {
		return nil, errors.New("png: error decoding thumbnail: " + err.Error())
	}

	return d.GenerateThumbnailOf(src, width, height, method, ctx)
}

func (d pngGenerator) GenerateThumbnailOf(src image.Image, width int, height int, method string, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	thumb, err := d.GenerateThumbnailImageOf(src, width, height, method, ctx)
	if err != nil || thumb == nil {
		return nil, err
	}

	imgData := &bytes.Buffer{}
	err = imaging.Encode(imgData, thumb, imaging.PNG)
	if err != nil {
		return nil, errors.New("png: error encoding thumbnail: " + err.Error())
	}
	return &m.Thumbnail{
		Animated:    false,
		ContentType: "image/png",
		Reader:      ioutil.NopCloser(imgData),
	}, nil
}

func (d pngGenerator) GenerateThumbnailImageOf(src image.Image, width int, height int, method string, ctx rcontext.RequestContext) (image.Image, error) {
	var shouldThumbnail bool
	shouldThumbnail, width, height, _, method = u.AdjustProperties(src, width, height, false, false, method)
	if !shouldThumbnail {
		return nil, nil
	}

	return u.MakeThumbnail(src, method, width, height)
}

func init() {
	generators = append(generators, pngGenerator{})
}
