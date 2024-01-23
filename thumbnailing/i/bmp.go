package i

import (
	"errors"
	"io"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/thumbnailing/m"
	"github.com/t2bot/matrix-media-repo/util"
	"golang.org/x/image/bmp"
)

type bmpGenerator struct {
}

func (d bmpGenerator) supportedContentTypes() []string {
	return []string{"image/bmp", "image/x-bmp"}
}

func (d bmpGenerator) supportsAnimation() bool {
	return false
}

func (d bmpGenerator) matches(img io.Reader, contentType string) bool {
	return util.ArrayContains(d.supportedContentTypes(), contentType)
}

func (d bmpGenerator) GetOriginDimensions(b io.Reader, contentType string, ctx rcontext.RequestContext) (bool, int, int, error) {
	i, err := bmp.DecodeConfig(b)
	if err != nil {
		return false, 0, 0, err
	}
	return true, i.Width, i.Height, nil
}

func (d bmpGenerator) GenerateThumbnail(b io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	src, err := bmp.Decode(b)
	if err != nil {
		return nil, errors.New("bmp: error decoding thumbnail: " + err.Error())
	}

	return pngGenerator{}.GenerateThumbnailOf(src, width, height, method, ctx)
}

func init() {
	generators = append(generators, bmpGenerator{})
}
