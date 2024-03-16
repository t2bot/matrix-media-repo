package preview

import (
	"fmt"
	"io"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"golang.org/x/image/tiff"
)

type tiffGenerator struct{}

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

func (d tiffGenerator) GenerateThumbnail(b io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*Thumbnail, error) {
	src, err := tiff.Decode(b)
	if err != nil {
		return nil, fmt.Errorf("tiff: error decoding thumbnail: %w", err)
	}

	return pngGenerator{}.GenerateThumbnailOf(src, width, height, method, ctx)
}

func init() {
	generators = append(generators, tiffGenerator{})
}
