package preview

import (
	"fmt"
	"image"
	_ "image/jpeg"
	"io"
	"slices"

	"github.com/disintegration/imaging"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/thumbnailing/preview/metadata"
	"github.com/t2bot/matrix-media-repo/util/readers"
)

type jpgGenerator struct{}

func (d jpgGenerator) supportedContentTypes() []string {
	return []string{"image/jpeg", "image/jpg"}
}

func (d jpgGenerator) supportsAnimation() bool {
	return false
}

func (d jpgGenerator) matches(img io.Reader, contentType string) bool {
	return slices.Contains(d.supportedContentTypes(), contentType)
}

func (d jpgGenerator) GetOriginDimensions(b io.Reader, contentType string, ctx rcontext.RequestContext) (bool, int, int, error) {
	return pngGenerator{}.GetOriginDimensions(b, contentType, ctx)
}

func (d jpgGenerator) GenerateThumbnail(b io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*Thumbnail, error) {
	br := readers.NewBufferReadsReader(b)
	orientation := metadata.ExtractExifOrientation(br)
	b = br.GetRewoundReader()

	src, err := imaging.Decode(b)
	if err != nil {
		return nil, fmt.Errorf("jpg: error decoding thumbnail: %w", err)
	}

	thumb, err := metadata.MakeThumbnail(src, method, width, height)
	if err != nil {
		return nil, fmt.Errorf("jpg: error making thumbnail: %w", err)
	}

	thumb = metadata.ApplyOrientation(thumb, orientation)

	pr, pw := io.Pipe()
	go func(pw *io.PipeWriter, p image.Image) {
		err = metadata.Encode(ctx, pw, p, metadata.JpegSource)
		if err != nil {
			_ = pw.CloseWithError(fmt.Errorf("jpg: error encoding thumbnail: %w", err))
		} else {
			_ = pw.Close()
		}
	}(pw, thumb)

	return &Thumbnail{
		Animated:    false,
		ContentType: "image/jpeg",
		Reader:      pr,
	}, nil
}

func init() {
	generators = append(generators, jpgGenerator{})
}
