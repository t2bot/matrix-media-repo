package i

import (
	"errors"
	"image"
	"image/draw"
	"image/gif"
	"io"
	"math"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/thumbnailing/m"
	"github.com/t2bot/matrix-media-repo/thumbnailing/u"
)

type gifGenerator struct {
}

func (d gifGenerator) supportedContentTypes() []string {
	return []string{"image/gif"}
}

func (d gifGenerator) supportsAnimation() bool {
	return true
}

func (d gifGenerator) matches(img io.Reader, contentType string) bool {
	return contentType == "image/gif"
}

func (d gifGenerator) GetOriginDimensions(b io.Reader, contentType string, ctx rcontext.RequestContext) (bool, int, int, error) {
	return pngGenerator{}.GetOriginDimensions(b, contentType, ctx)
}

func (d gifGenerator) GenerateThumbnail(b io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	g, err := gif.DecodeAll(b)
	if err != nil {
		return nil, errors.New("gif: error decoding image: " + err.Error())
	}

	// Prepare a blank frame to use as swap space
	frameImg := image.NewRGBA(image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: g.Config.Width, Y: g.Config.Height}})

	targetStaticFrame := int(math.Floor(math.Min(1, math.Max(0, float64(ctx.Config.Thumbnails.StillFrame))) * float64(len(g.Image))))

	for i, img := range g.Image {
		var disposal byte
		// use disposal method 0 by default
		if g.Disposal == nil {
			disposal = 0
		} else {
			disposal = g.Disposal[i]
		}

		// Copy the frame to a new image and use that
		draw.Draw(frameImg, frameImg.Bounds(), img, image.Point{X: 0, Y: 0}, draw.Over)

		// Do the thumbnailing on the copied frame
		frameThumb, err := u.MakeThumbnail(frameImg, method, width, height)
		if err != nil {
			return nil, errors.New("gif: error generating thumbnail frame: " + err.Error())
		}
		if frameThumb == nil {
			tmpImg := image.NewRGBA(frameImg.Bounds())
			draw.Draw(tmpImg, tmpImg.Bounds(), frameImg, image.Point{X: 0, Y: 0}, draw.Src)
			frameThumb = tmpImg
		}

		targetImg := image.NewPaletted(frameThumb.Bounds(), img.Palette)
		draw.FloydSteinberg.Draw(targetImg, frameThumb.Bounds(), frameThumb, image.Point{X: 0, Y: 0})

		if !animated && i == targetStaticFrame {
			t, err := pngGenerator{}.GenerateThumbnailOf(targetImg, width, height, method, ctx)
			if err != nil || t != nil {
				return t, err
			}

			// The thumbnailer decided that it shouldn't thumbnail, so encode it ourselves
			pr, pw := io.Pipe()
			go func(pw *io.PipeWriter, p *image.Paletted) {
				err = u.Encode(ctx, pw, p)
				if err != nil {
					_ = pw.CloseWithError(errors.New("gif: error encoding still frame thumbnail: " + err.Error()))
				} else {
					_ = pw.Close()
				}
			}(pw, targetImg)
			return &m.Thumbnail{
				Animated:    false,
				ContentType: "image/png",
				Reader:      pr,
			}, nil
		}

		// if disposal type is 0 or 1 (preserve previous frame) we can get artifacts from re-scaling.
		// as such, we re-render those frames to disposal type 1 (do not dispose)
		// Importantly, we do not clear the previous frame buffer canvas
		// see https://www.w3.org/Graphics/GIF/spec-gif89a.txt
		// This also applies to frame disposal type 0, https://legacy.imagemagick.org/Usage/anim_basics/#none
		if disposal == 1 || disposal == 0 {
			g.Disposal[i] = 1
		} else {
			draw.Draw(frameImg, frameImg.Bounds(), image.Transparent, image.Point{X: 0, Y: 0}, draw.Src)
		}

		g.Image[i] = targetImg
	}

	// Set the image size to the first frame's size
	g.Config.Width = g.Image[0].Bounds().Max.X
	g.Config.Height = g.Image[0].Bounds().Max.Y

	pr, pw := io.Pipe()
	go func(pw *io.PipeWriter, g *gif.GIF) {
		err = gif.EncodeAll(pw, g)
		if err != nil {
			_ = pw.CloseWithError(errors.New("gif: error encoding final thumbnail: " + err.Error()))
		} else {
			_ = pw.Close()
		}
	}(pw, g)

	return &m.Thumbnail{
		ContentType: "image/gif",
		Animated:    true,
		Reader:      pr,
	}, nil
}

func init() {
	generators = append(generators, gifGenerator{})
}
