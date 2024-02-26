package i

import (
	"errors"
	"image"
	_ "image/jpeg"
	"io"
	"slices"

	"github.com/disintegration/imaging"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/thumbnailing/m"
	"github.com/t2bot/matrix-media-repo/thumbnailing/u"
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

func (d jpgGenerator) GenerateThumbnail(b io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	br := readers.NewBufferReadsReader(b)
	orientation := u.ExtractExifOrientation(br)
	b = br.GetRewoundReader()

	src, err := imaging.Decode(b)
	if err != nil {
		return nil, errors.New("jpg: error decoding thumbnail: " + err.Error())
	}

	thumb, err := u.MakeThumbnail(src, method, width, height)
	if err != nil {
		return nil, errors.New("jpg: error making thumbnail: " + err.Error())
	}

	thumb = u.ApplyOrientation(thumb, orientation)

	pr, pw := io.Pipe()
	go func(pw *io.PipeWriter, p image.Image) {
		err = u.Encode(ctx, pw, p, u.JpegSource)
		if err != nil {
			_ = pw.CloseWithError(errors.New("jpg: error encoding thumbnail: " + err.Error()))
		} else {
			_ = pw.Close()
		}
	}(pw, thumb)

	return &m.Thumbnail{
		Animated:    false,
		ContentType: "image/jpeg",
		Reader:      pr,
	}, nil
}

func init() {
	generators = append(generators, jpgGenerator{})
}
