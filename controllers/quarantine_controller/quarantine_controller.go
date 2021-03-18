package quarantine_controller

import (
	"bytes"
	"github.com/getsentry/sentry-go"
	"image"
	"image/color"
	"math"

	"github.com/disintegration/imaging"
	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"golang.org/x/image/font/gofont/gosmallcaps"
)

func GenerateQuarantineThumbnail(width int, height int, ctx rcontext.RequestContext) (image.Image, error) {
	var centerImage image.Image
	var err error
	if ctx.Config.Quarantine.ThumbnailPath != "" {
		centerImage, err = imaging.Open(ctx.Config.Quarantine.ThumbnailPath)
	} else {
		centerImage, err = generateDefaultQuarantineThumbnail()
	}
	if err != nil {
		return nil, err
	}

	c := gg.NewContext(width, height)

	centerImage = imaging.Fit(centerImage, width, height, imaging.Lanczos)

	c.DrawImageAnchored(centerImage, width/2, height/2, 0.5, 0.5)

	buf := &bytes.Buffer{}
	c.EncodePNG(buf)

	return imaging.Decode(buf)
}

func generateDefaultQuarantineThumbnail() (image.Image, error) {
	c := gg.NewContext(700, 700)
	c.Clear()

	red := color.RGBA{R: 190, G: 26, B: 25, A: 255}
	orange := color.RGBA{R: 255, G: 186, B: 73, A: 255}
	x := 350.0
	y := 300.0
	r := 256.0
	w := 55.0
	p := 64.0
	m := "media not allowed"

	c.SetColor(orange)
	c.DrawRectangle(0, 0, 700, 700)
	c.Fill()

	c.SetColor(red)
	c.DrawCircle(x, y, r)
	c.Fill()

	c.SetColor(color.White)
	c.DrawCircle(x, y, r-w)
	c.Fill()

	lr := r - (w / 2)
	sx := x + (lr * math.Cos(gg.Radians(225.0)))
	sy := y + (lr * math.Sin(gg.Radians(225.0)))
	ex := x + (lr * math.Cos(gg.Radians(45.0)))
	ey := y + (lr * math.Sin(gg.Radians(45.0)))
	c.SetLineCap(gg.LineCapButt)
	c.SetLineWidth(w)
	c.SetColor(red)
	c.DrawLine(sx, sy, ex, ey)
	c.Stroke()

	f, err := truetype.Parse(gosmallcaps.TTF)
	if err != nil {
		sentry.CaptureException(err)
		panic(err)
	}

	c.SetColor(color.Black)
	c.SetFontFace(truetype.NewFace(f, &truetype.Options{Size: 64}))
	c.DrawStringAnchored(m, x, y+r+p, 0.5, 0.5)

	buf := &bytes.Buffer{}
	c.EncodePNG(buf)

	return imaging.Decode(buf)
}
