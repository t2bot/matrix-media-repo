package i

import (
	"errors"
	"io"

	"github.com/davidbyttow/govips/v2/vips"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/thumbnailing/m"
	"github.com/t2bot/matrix-media-repo/thumbnailing/u"
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
	// Fix: animated webp support
	buf, err := io.ReadAll(b)
	if err != nil {
		return nil, errors.New("thumbnail: error reading data: " + err.Error())
	}
	p := &vips.ImportParams{}
	if animated {
		p.NumPages.Set(-1) // Set parameter for libvip loader to support more than one page
	} else {
		p.NumPages.Set(1) // Reduce page loading
	}

	i, err := vips.LoadImageFromBuffer(buf, p)
	if err != nil {
		return nil, errors.New("vips: error decoding: " + err.Error())
	}

	return d.GenerateThumbnailOf(i, width, height, method, animated, ctx)
}

func (d webpGenerator) GenerateThumbnailOf(i *vips.ImageRef, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	tb, err := u.MakeThumbnailByVips(i, method, width, height, animated)
	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	go func(pw *io.PipeWriter, bt []byte) {
		_, err := pw.Write(bt)
		if err != nil {
			_ = pw.CloseWithError(errors.New("webp: error loading thumbnail data: " + err.Error()))
		} else {
			_ = pw.Close()
		}
	}(pw, tb)

	return &m.Thumbnail{
		Animated:    animated,
		ContentType: "image/webp",
		Reader:      pr,
	}, nil
}

func init() {
	generators = append(generators, webpGenerator{})
}
