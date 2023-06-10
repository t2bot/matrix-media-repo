package i

import (
	"errors"
	"io"

	"github.com/adrium/goheif"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/thumbnailing/m"
)

type heifGenerator struct {
}

func (d heifGenerator) supportedContentTypes() []string {
	return []string{"image/heif"}
}

func (d heifGenerator) supportsAnimation() bool {
	return true
}

func (d heifGenerator) matches(img io.Reader, contentType string) bool {
	return contentType == "image/heif"
}

func (d heifGenerator) GetOriginDimensions(b io.Reader, contentType string, ctx rcontext.RequestContext) (bool, int, int, error) {
	i, err := goheif.DecodeConfig(b)
	if err != nil {
		return false, 0, 0, err
	}
	return true, i.Width, i.Height, nil
}

func (d heifGenerator) GenerateThumbnail(b io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	// Use more memory, but prevent crashes
	goheif.SafeEncoding = true

	src, err := goheif.Decode(b)
	if err != nil {
		return nil, errors.New("heif: error decoding thumbnail: " + err.Error())
	}

	return pngGenerator{}.GenerateThumbnailOf(src, width, height, method, ctx)
}

func init() {
	generators = append(generators, heifGenerator{})
}
