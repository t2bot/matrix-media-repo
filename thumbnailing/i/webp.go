package i

import (
	"bytes"
	"errors"
	"image"
	"io"

	"github.com/davidbyttow/govips/v2/vips"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/thumbnailing/m"
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
	i, err := vips.NewImageFromReader(b)
	if err != nil {
		return false, 0, 0, err
	}
	m := i.Metadata()
	return true, m.Width, m.Height, nil
}

func (d webpGenerator) GenerateThumbnail(b io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	i, err := vips.NewImageFromReader(b)
	if err != nil {
		return nil, errors.New("vips: error decoding: " + err.Error())
	}
	data, _, err := i.ExportPng(&vips.PngExportParams{StripMetadata: true})
	if err != nil {
		return nil, errors.New("vips: error when preprocessing the file: " + err.Error())
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("webp: error decoding thumbnail: " + err.Error())
	}

	return pngGenerator{}.GenerateThumbnailOf(src, width, height, method, ctx)
}

func init() {
	generators = append(generators, webpGenerator{})
}
