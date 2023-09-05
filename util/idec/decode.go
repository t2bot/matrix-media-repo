package idec

import (
	"image"
	"io"

	"github.com/viam-labs/go-libjpeg/jpeg"

	"github.com/disintegration/imaging"
	"github.com/turt2live/matrix-media-repo/util/readers"
)

func DecodeConfig(r io.Reader) (image.Config, error) {
	br := readers.NewBufferReadsReader(r)
	c, err := jpeg.DecodeConfig(br)
	b := br.GetRewoundReader()
	if err == nil {
		return c, nil
	}
	c, _, err = image.DecodeConfig(b)
	return c, err
}

func Decode(r io.Reader) (image.Image, error) {
	br := readers.NewBufferReadsReader(r)
	_, err := jpeg.DecodeConfig(br)
	b := br.GetRewoundReader()
	if err == nil {
		return jpeg.Decode(b, &jpeg.DecoderOptions{})
	}
	return imaging.Decode(r)
}
