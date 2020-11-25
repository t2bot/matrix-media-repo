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
	return []string{"image/png", "image/apng"}
}

func (d apngGenerator) supportsAnimation() bool {
	return true
}

func (d apngGenerator) matches(img []byte, contentType string) bool {
	return (contentType == "image/png" && util.IsAnimatedPNG(img)) || contentType == "image/apng"
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

		// we must make sure that blend op, frame disposal etc. fit together correctly.
		// as we re-render all frames at (0, 0) we need to keep a swap space of the preivous image
		// For which blend op etc. is what, see https://wiki.mozilla.org/APNG_Specification#.60fcTL.60:_The_Frame_Control_Chunk
		if p.Frames[i].DisposeOp == apng.DISPOSE_OP_BACKGROUND || p.Frames[i].BlendOp == apng.BLEND_OP_OVER {
			// Clear the transparency of the previous frame
			draw.Draw(frameImg, frameImg.Bounds(), image.Transparent, image.Point{X: 0, Y: 0}, draw.Src)
		}

		// preserve our frame, if the dispose method is previous
		tmpImg := image.NewRGBA(frameImg.Bounds())
		if p.Frames[i].DisposeOp == apng.DISPOSE_OP_PREVIOUS {
			draw.Draw(tmpImg, frameImg.Bounds(), frameImg, image.Point{X: 0, Y: 0}, draw.Src)
		}

		// Copy the frame to a new image and use that
		draw.Draw(frameImg, image.Rect(frame.XOffset, frame.YOffset, frameImg.Rect.Max.X, frameImg.Rect.Max.Y), img, image.Point{X: 0, Y: 0}, draw.Src)

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

		// restore the frame, if the dispose method is previous
		if p.Frames[i].DisposeOp == apng.DISPOSE_OP_PREVIOUS {
			draw.Draw(frameImg, frameImg.Bounds(), tmpImg, image.Point{X: 0, Y: 0}, draw.Src)
		}
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
