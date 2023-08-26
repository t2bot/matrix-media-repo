package jpeg2

import (
	"image"
	gojpeg "image/jpeg"
	"io"

	"github.com/turt2live/matrix-media-repo/util/readers"
	libjpeg "github.com/viam-labs/go-libjpeg/jpeg"
)

func Decode(r io.Reader) (image.Image, error) {
	br := readers.NewBufferReadsReader(r)
	c, err := libjpeg.Decode(r, &libjpeg.DecoderOptions{})
	if err != nil {
		r = br.GetRewoundReader()
		return gojpeg.Decode(r)
	} else {
		br.Discard()
	}
	return c, nil
}

func DecodeConfig(r io.Reader) (image.Config, error) {
	br := readers.NewBufferReadsReader(r)
	c, err := libjpeg.DecodeConfig(r)
	if err != nil {
		r = br.GetRewoundReader()
		return gojpeg.DecodeConfig(r)
	} else {
		br.Discard()
	}
	return c, nil
}

func init() {
	// We get registered "first", overriding which jpeg decoder is used
	image.RegisterFormat("jpeg", "\xff\xd8", Decode, DecodeConfig)
}
