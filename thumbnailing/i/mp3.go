package i

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"io"
	"math"
	"os"
	"path"

	"github.com/dhowden/tag"
	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/thumbnailing/m"
	"github.com/t2bot/matrix-media-repo/thumbnailing/u"
	"github.com/t2bot/matrix-media-repo/util/readers"
)

type mp3Generator struct {
}

func (d mp3Generator) supportedContentTypes() []string {
	return []string{"audio/mpeg"}
}

func (d mp3Generator) supportsAnimation() bool {
	return false
}

func (d mp3Generator) matches(img io.Reader, contentType string) bool {
	return contentType == "audio/mpeg"
}

func (d mp3Generator) decode(b io.Reader) (beep.StreamSeekCloser, beep.Format, error) {
	audio, format, err := mp3.Decode(readers.MakeCloser(b))
	if err != nil {
		return audio, format, errors.New("mp3: error decoding audio: " + err.Error())
	}
	return audio, format, nil
}

func (d mp3Generator) GetOriginDimensions(b io.Reader, contentType string, ctx rcontext.RequestContext) (bool, int, int, error) {
	return false, 0, 0, nil
}

func (d mp3Generator) GenerateThumbnail(b io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	tags, rc, err := u.GetID3Tags(b)
	if err != nil {
		return nil, errors.New("mp3: error getting tags: " + err.Error())
	}
	//goland:noinspection GoUnhandledErrorResult
	defer rc.Close()

	audio, format, err := d.decode(rc)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer audio.Close()
	return d.GenerateFromStream(audio, format, tags, width, height, ctx)
}

func (d mp3Generator) GetAudioData(b io.Reader, nKeys int, ctx rcontext.RequestContext) (*m.AudioInfo, error) {
	audio, format, err := d.decode(b)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer audio.Close()
	return d.GetDataFromStream(audio, format, nKeys)
}

func (d mp3Generator) GetDataFromStream(audio beep.StreamSeekCloser, format beep.Format, nKeys int) (*m.AudioInfo, error) {
	totalSamples := audio.Len()
	downsampled, err := u.FastSampleAudio(audio, nKeys)
	if err != nil {
		return nil, err
	}

	return &m.AudioInfo{
		Duration:     format.SampleRate.D(totalSamples),
		Channels:     format.NumChannels,
		TotalSamples: totalSamples,
		KeySamples:   downsampled,
	}, nil
}

func (d mp3Generator) GenerateFromStream(audio beep.StreamSeekCloser, format beep.Format, meta tag.Metadata, width int, height int, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	bgColor := color.RGBA{A: 255, R: 41, G: 57, B: 92}
	fgColor := color.RGBA{A: 255, R: 240, G: 240, B: 240}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	padding := 16

	sq := int(math.Round(float64(height) * 0.66))
	var artworkImg image.Image
	if meta != nil && meta.Picture() != nil {
		artwork, _, _ := image.Decode(bytes.NewBuffer(meta.Picture().Data))
		if artwork != nil {
			artworkImg, _ = u.MakeThumbnail(artwork, "crop", sq, sq)
		}
	}

	ax := sq
	ay := sq

	if artworkImg != nil {
		ax = artworkImg.Bounds().Max.X
		ay = artworkImg.Bounds().Max.Y
	}

	dy := (height / 2) - (ay / 2)
	dx := padding
	ddy := ay + dy
	ddx := ax + dx
	r := image.Rect(dx, dy, ddx, ddy)

	if artworkImg == nil {
		f, _ := os.OpenFile(path.Join(config.Runtime.AssetsPath, "default-artwork.png"), os.O_RDONLY, 0640)
		if f != nil {
			defer f.Close()
			tmp, _, _ := image.Decode(f)
			if tmp != nil {
				artworkImg, _ = u.MakeThumbnail(tmp, "crop", ax, ay)
			}
		}
		if artworkImg == nil {
			logrus.Warn("Falling back to black square for artwork")
			tmp := image.NewRGBA(image.Rect(0, 0, ax, ay))
			for x := 0; x < tmp.Bounds().Max.X; x++ {
				for y := 0; y < tmp.Bounds().Max.Y; y++ {
					tmp.Set(x, y, color.Black)
				}
			}
			artworkImg = tmp
		}
	}

	draw.Draw(img, r, artworkImg, image.Pt(0, 0), draw.Over)

	waveformX := padding + r.Max.X
	info, err := d.GetDataFromStream(audio, format, (int)(math.Max((float64)(width-waveformX-padding), 1)))
	if err != nil {
		return nil, errors.New("beep-visual: error sampling audio: " + err.Error())
	}

	// Average out all the samples
	averagedSamples := make([]float64, 0)
	for _, s := range info.KeySamples {
		avg := (s[0] + s[1]) / 2
		if info.Channels == 1 {
			avg = s[0]
		}
		averagedSamples = append(averagedSamples, avg)
	}

	// Now that we have samples and artwork, generate a plot
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
			col := bgColor
			isWithin := y <= center && y >= px
			if !above {
				isWithin = y >= center && y <= px
			}
			if isWithin {
				col = fgColor
			}
			img.Set(x+waveformX, y, col)
		}
	}

	// Fill in the background
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			c := img.RGBAAt(x, y)
			if c.A == 0 {
				img.Set(x, y, bgColor)
			}
		}
	}

	// Encode to a png
	pr, pw := io.Pipe()
	go func(pw *io.PipeWriter, p image.Image) {
		err = u.Encode(ctx, pw, p)
		if err != nil {
			_ = pw.CloseWithError(errors.New("beep-visual: error encoding thumbnail: " + err.Error()))
		} else {
			_ = pw.Close()
		}
	}(pw, img)

	return &m.Thumbnail{
		Animated:    false,
		ContentType: "image/png",
		Reader:      pr,
	}, nil
}

func init() {
	generators = append(generators, mp3Generator{})
}
