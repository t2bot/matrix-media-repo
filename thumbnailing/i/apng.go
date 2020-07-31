package i

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"io/ioutil"

	"github.com/kettek/apng"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/thumbnailing/m"
	"github.com/turt2live/matrix-media-repo/util"
)

type apngGenerator struct {
}

func (d apngGenerator) supportedContentTypes() []string {
	return []string{"image/png"}
}

func (d apngGenerator) supportsAnimation() bool {
	return true
}

func (d apngGenerator) matches(img []byte, contentType string) bool {
	return contentType == "image/png" && util.IsAnimatedPNG(img)
}

func (d apngGenerator) GenerateThumbnail(b []byte, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	if !animated {
		return pngGenerator{}.GenerateThumbnail(b, "image/png", width, height, method, false, ctx)
	}

	p, err := apng.DecodeAll(bytes.NewBuffer(b))
	if err != nil {
		return nil, errors.New("apng: error decoding image: " + err.Error())
	}

	// prepare a blank frame to use as swap space
	frameImg := image.NewRGBA(p.Frames[0].Image.Bounds())

	for i, frame := range p.Frames {
		img := frame.Image

		// Clear the transparency of the previous frame
		draw.Draw(frameImg, frameImg.Bounds(), image.Transparent, image.Point{X: 0, Y: 0}, draw.Src)

		// Copy the frame to a new image and use that
		draw.Draw(frameImg, image.Rect(frame.XOffset, frame.YOffset, frameImg.Rect.Max.X, frameImg.Rect.Max.Y), img, image.Point{X: 0, Y: 0}, draw.Over)

		// Do the thumbnailing on the copied frame
		frameThumb, err := pngGenerator{}.GenerateThumbnailImageOf(frameImg, width, height, method, ctx)
		if err != nil {
			return nil, errors.New("apng: error generating thumbnail frame: " + err.Error())
		}
		if frameThumb == nil {
			tmpImg := image.NewRGBA(frameImg.Bounds())
			draw.Draw(tmpImg, tmpImg.Bounds(), frameImg, image.Point{X: 0, Y: 0}, draw.Src)
			frameThumb = tmpImg
		}

		p.Frames[i].Image = frameThumb
		p.Frames[i].XOffset = 0
		p.Frames[i].YOffset = 0
	}

	buf := &bytes.Buffer{}
	err = apng.Encode(buf, p)
	if err != nil {
		return nil, errors.New("apng: error encoding final thumbnail: " + err.Error())
	}

	return &m.Thumbnail{
		ContentType: "image/png",
		Animated:    true,
		Reader:      ioutil.NopCloser(buf),
	}, nil
}

func init() {
	generators = append(generators, apngGenerator{})
}
