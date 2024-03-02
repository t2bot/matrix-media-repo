package preview

import (
	"fmt"
	"image"
	"image/draw"
	"io"

	"github.com/getsentry/sentry-go"
	"github.com/kettek/apng"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/thumbnailing/preview/metadata"
)

type apngGenerator struct{}

func (d apngGenerator) supportedContentTypes() []string {
	return []string{"image/png", "image/apng"}
}

func (d apngGenerator) supportsAnimation() bool {
	return true
}

func (d apngGenerator) matches(img io.Reader, contentType string) bool {
	return (contentType == "image/png" && isAnimatedPNG(img)) || contentType == "image/apng"
}

func (d apngGenerator) GetOriginDimensions(b io.Reader, contentType string, ctx rcontext.RequestContext) (bool, int, int, error) {
	i, err := apng.DecodeConfig(b)
	if err != nil {
		return false, 0, 0, err
	}
	return true, i.Width, i.Height, nil
}

func (d apngGenerator) GenerateThumbnail(b io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*Thumbnail, error) {
	if !animated {
		return pngGenerator{}.GenerateThumbnail(b, "image/png", width, height, method, false, ctx)
	}

	p, err := apng.DecodeAll(b)
	if err != nil {
		return nil, fmt.Errorf("apng: error decoding image: %w", err)
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
		frameThumb, err := metadata.MakeThumbnail(frameImg, method, width, height)
		if err != nil {
			return nil, fmt.Errorf("apng: error generating thumbnail frame: %w", err)
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

	pr, pw := io.Pipe()
	go func(pw *io.PipeWriter, p apng.APNG) {
		err = apng.Encode(pw, p)
		if err != nil {
			_ = pw.CloseWithError(fmt.Errorf("apng: error encoding final thumbnail: %w", err))
		} else {
			_ = pw.Close()
		}
	}(pw, p)

	return &Thumbnail{
		ContentType: "image/png",
		Animated:    true,
		Reader:      pr,
	}, nil
}

func init() {
	generators = append(generators, apngGenerator{})
}

func isAnimatedPNG(r io.Reader) bool {
	maxBytes := 4096 // if we don't have an acTL chunk after 4kb, give up
	IDAT := []byte{0x49, 0x44, 0x41, 0x54}
	acTL := []byte{0x61, 0x63, 0x54, 0x4C}

	b := make([]byte, maxBytes)
	c, err := r.Read(b)
	if err != nil {
		// we don't log the error, but we do want to report it if sentry is hooked up
		sentry.CaptureException(err)
		return false // assume read errors are a problem
	}

	idatIdx := 0
	actlIdx := 0
	for i, bt := range b {
		if i > c {
			break
		}
		if bt == IDAT[idatIdx] {
			idatIdx++
			actlIdx = 0
		} else if bt == acTL[actlIdx] {
			actlIdx++
			idatIdx = 0
		} else {
			idatIdx = 0
			actlIdx = 0
		}

		if idatIdx == len(IDAT) {
			return false
		}
		if actlIdx == len(acTL) {
			return true
		}
	}

	return false
}
