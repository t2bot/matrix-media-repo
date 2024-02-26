package i

import (
	"fmt"
	"image"
	"io"
	"slices"

	_ "github.com/strukturag/libheif/go/heif"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/thumbnailing/m"
)

type heifGenerator struct{}

func (d heifGenerator) supportedContentTypes() []string {
	return []string{"image/heif", "image/heic"}
}

func (d heifGenerator) supportsAnimation() bool {
	return true
}

func (d heifGenerator) matches(img io.Reader, contentType string) bool {
	return slices.Contains(d.supportedContentTypes(), contentType)
}

func (d heifGenerator) GetOriginDimensions(b io.Reader, contentType string, ctx rcontext.RequestContext) (bool, int, int, error) {
	cfg, _, err := image.DecodeConfig(b)
	if err != nil {
		return false, 0, 0, err
	}
	return true, cfg.Width, cfg.Height, nil
}

func (d heifGenerator) GenerateThumbnail(b io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	src, _, err := image.Decode(b)
	if err != nil {
		return nil, fmt.Errorf("heif: error decoding thumbnail: %w", err)
	}

	return pngGenerator{}.GenerateThumbnailOf(src, width, height, method, ctx)
}

func init() {
	generators = append(generators, heifGenerator{})
}
