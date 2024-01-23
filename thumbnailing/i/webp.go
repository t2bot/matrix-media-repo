package i

import (
	"errors"
	"io"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/thumbnailing/m"
	"golang.org/x/image/webp"
)

type webpGenerator struct {
}

func (d webpGenerator) supportedContentTypes() []string {
	return []string{"image/webp"}
}

func (d webpGenerator) supportsAnimation() bool {
	return true
}

func (d webpGenerator) matches(img io.Reader, contentType string) bool {
	return contentType == "image/webp"
}

func (d webpGenerator) GetOriginDimensions(b io.Reader, contentType string, ctx rcontext.RequestContext) (bool, int, int, error) {
	i, err := webp.DecodeConfig(b)
	if err != nil {
		return false, 0, 0, err
	}
	return true, i.Width, i.Height, nil
}

func (d webpGenerator) GenerateThumbnail(b io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	src, err := webp.Decode(b)
	if err != nil {
		return nil, errors.New("webp: error decoding thumbnail: " + err.Error())
	}

	return pngGenerator{}.GenerateThumbnailOf(src, width, height, method, ctx)
}

func init() {
	generators = append(generators, webpGenerator{})
}
