package i

import (
	"errors"
	"image"
	_ "image/png"
	"io"

	"github.com/disintegration/imaging"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/thumbnailing/m"
	"github.com/t2bot/matrix-media-repo/thumbnailing/u"
)

type pngGenerator struct {
}

func (d pngGenerator) supportedContentTypes() []string {
	return []string{"image/png"}
}

func (d pngGenerator) supportsAnimation() bool {
	return false
}

func (d pngGenerator) matches(img io.Reader, contentType string) bool {
	return contentType == "image/png"
}

func (d pngGenerator) GetOriginDimensions(b io.Reader, contentType string, ctx rcontext.RequestContext) (bool, int, int, error) {
	i, _, err := image.DecodeConfig(b)
	if err != nil {
		return false, 0, 0, err
	}
	return true, i.Width, i.Height, nil
}

func (d pngGenerator) GenerateThumbnail(b io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	src, err := imaging.Decode(b)
	if err != nil {
		return nil, errors.New("png: error decoding thumbnail: " + err.Error())
	}

	return d.GenerateThumbnailOf(src, width, height, method, ctx)
}

func (d pngGenerator) GenerateThumbnailOf(src image.Image, width int, height int, method string, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	thumb, err := u.MakeThumbnail(src, method, width, height)
	if err != nil || thumb == nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	go func(pw *io.PipeWriter, p image.Image) {
		err = u.Encode(ctx, pw, p)
		if err != nil {
			_ = pw.CloseWithError(errors.New("png: error encoding thumbnail: " + err.Error()))
		} else {
			_ = pw.Close()
		}
	}(pw, thumb)

	return &m.Thumbnail{
		Animated:    false,
		ContentType: "image/png",
		Reader:      pr,
	}, nil
}

func init() {
	generators = append(generators, pngGenerator{})
}
