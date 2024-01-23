package i

import (
	"errors"
	"io"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/thumbnailing/m"
	"golang.org/x/image/tiff"
)

type tiffGenerator struct {
}

func (d tiffGenerator) supportedContentTypes() []string {
	return []string{"image/tiff"}
}

func (d tiffGenerator) supportsAnimation() bool {
	return false
}

func (d tiffGenerator) matches(img io.Reader, contentType string) bool {
	return contentType == "image/tiff"
}

func (d tiffGenerator) GetOriginDimensions(b io.Reader, contentType string, ctx rcontext.RequestContext) (bool, int, int, error) {
	i, err := tiff.DecodeConfig(b)
	if err != nil {
		return false, 0, 0, err
	}
	return true, i.Width, i.Height, nil
}

func (d tiffGenerator) GenerateThumbnail(b io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	src, err := tiff.Decode(b)
	if err != nil {
		return nil, errors.New("tiff: error decoding thumbnail: " + err.Error())
	}

	return pngGenerator{}.GenerateThumbnailOf(src, width, height, method, ctx)
}

func init() {
	generators = append(generators, tiffGenerator{})
}
