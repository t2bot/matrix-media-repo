package test_internals

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"io"
	"testing"

	"github.com/disintegration/imaging"
	"github.com/stretchr/testify/assert"
)

var evenColor = color.RGBA{R: 255, G: 0, B: 0, A: 255}
var oddColor = color.RGBA{R: 0, G: 255, B: 0, A: 255}
var altColor = color.RGBA{R: 0, G: 0, B: 255, A: 255}

func colorFor(x int, y int) color.Color {
	c := oddColor
	if (y%2.0) == 0 && (x%2.0) == 0 {
		c = altColor
	} else if (y%2.0) == 0 || (x%2.0) == 0 {
		c = evenColor
	}
	return c
}

func MakeTestImage(width int, height int) (string, io.Reader, error) {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			c := colorFor(x, y)
			img.Set(x, y, c)
		}
	}

	b := bytes.NewBuffer(make([]byte, 0))
	err := imaging.Encode(b, img, imaging.PNG)
	if err != nil {
		return "", nil, err
	}

	return "image/png", b, nil
}

func AssertIsTestImage(t *testing.T, i io.Reader) {
	img, _, err := image.Decode(i)
	assert.NoError(t, err, "Error decoding image")
	width := img.Bounds().Max.X
	height := img.Bounds().Max.Y
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			c := colorFor(x, y)
			if !assert.Equal(t, c, img.At(x, y), fmt.Sprintf("Wrong colour for pixel %d,%d", x, y)) {
				return // don't print thousands of errors
			}
		}
	}
}
