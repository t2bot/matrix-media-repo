package i

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"io"
	"io/ioutil"
	"math"

	"github.com/disintegration/imaging"
	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/thumbnailing/m"
	"github.com/turt2live/matrix-media-repo/util"
)

type mp3Generator struct {
}

func (d mp3Generator) supportedContentTypes() []string {
	return []string{"audio/mpeg"}
}

func (d mp3Generator) supportsAnimation() bool {
	return false
}

func (d mp3Generator) matches(img []byte, contentType string) bool {
	return contentType == "audio/mpeg"
}

func (d mp3Generator) GenerateThumbnail(b []byte, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	audio, format, err := mp3.Decode(util.ByteCloser(b))
	if err != nil {
		return nil, errors.New("mp3: error decoding audio: " + err.Error())
	}

	defer audio.Close()
	return d.GenerateFromStream(audio, format, width, height)
}

func (d mp3Generator) GenerateFromStream(audio beep.StreamSeekCloser, format beep.Format, width int, height int) (*m.Thumbnail, error) {
	allSamples := make([][2]float64, 0)

	moreSamples := true
	samples := make([][2]float64, 100000) // a 3 minute mp3 has roughly 7 million samples, so reduce the number of iterations we have to do
	for moreSamples {
		n, ok := audio.Stream(*&samples)
		if n == 0 {
			moreSamples = false
		}
		if !ok && audio.Err() != nil && audio.Err() != io.EOF {
			return nil, errors.New("beep-visual: error sampling audio: " + audio.Err().Error())
		}
		for i, v := range samples {
			if i >= n {
				break
			}
			allSamples = append(allSamples, v)
		}
	}

	// Figure out a resolution that will work for us
	everyNth := int(math.Round(float64(len(allSamples)) / float64(width)))
	averagedSamples := make([]float64, 0)
	for i, s := range allSamples {
		if i%everyNth != 0 {
			continue
		}
		avg := (s[0] + s[1]) / 2
		if format.NumChannels == 1 {
			avg = s[0]
		}
		averagedSamples = append(averagedSamples, avg)
	}

	// Now that we have samples, generate a plot
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	padding := 16
	center := height / 2
	for x, s := range averagedSamples {
		distance := int(math.Round(float64((height-padding)/2) * math.Abs(s)))
		above := true
		px := center - distance
		if s < 0 {
			px = center + distance
			above = false
		}
		for y := 0; y < height; y++ {
			col := color.RGBA{A: 255, R: 41, G: 57, B: 92}
			isWithin := y <= center && y >= px
			if !above {
				isWithin = y >= center && y <= px
			}
			if isWithin {
				col = color.RGBA{A: 255, R: 240, G: 240, B: 240}
			}
			img.Set(x, y, col)
		}
	}

	// Encode to a png
	imgData := &bytes.Buffer{}
	err := imaging.Encode(imgData, img, imaging.PNG)
	if err != nil {
		return nil, errors.New("beep-visual: error encoding thumbnail: " + err.Error())
	}

	return &m.Thumbnail{
		Animated:    false,
		ContentType: "image/png",
		Reader:      ioutil.NopCloser(imgData),
	}, nil
}

func init() {
	generators = append(generators, mp3Generator{})
}
