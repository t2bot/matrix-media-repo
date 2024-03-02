package preview

import (
	"fmt"
	"image"
	_ "image/png"
	"io"

	"github.com/disintegration/imaging"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/thumbnailing/u"
)

type pngGenerator struct{}

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

func (d pngGenerator) GenerateThumbnail(b io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*Thumbnail, error) {
	src, err := imaging.Decode(b)
	if err != nil {
		return nil, fmt.Errorf("png: error decoding thumbnail: %w", err)
	}

	return d.GenerateThumbnailOf(src, width, height, method, ctx)
}

func (d pngGenerator) GenerateThumbnailOf(src image.Image, width int, height int, method string, ctx rcontext.RequestContext) (*Thumbnail, error) {
	thumb, err := u.MakeThumbnail(src, method, width, height)
	if err != nil || thumb == nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	go func(pw *io.PipeWriter, p image.Image) {
		err = u.Encode(ctx, pw, p)
		if err != nil {
			_ = pw.CloseWithError(fmt.Errorf("png: error encoding thumbnail: %w", err))
		} else {
			_ = pw.Close()
		}
	}(pw, thumb)

	return &Thumbnail{
		Animated:    false,
		ContentType: "image/png",
		Reader:      pr,
	}, nil
}

func init() {
	generators = append(generators, pngGenerator{})
}
