package jpeg2

import (
	"image"
	"io"

	libjpeg "github.com/viam-labs/go-libjpeg/jpeg"
)

func Decode(r io.Reader) (image.Image, error) {
	return libjpeg.Decode(r, &libjpeg.DecoderOptions{})
}

func DecodeConfig(r io.Reader) (image.Config, error) {
	return libjpeg.DecodeConfig(r)
}

func init() {
	// We get registered "first", overriding which jpeg decoder is used
	image.RegisterFormat("jpeg", "\xff\xd8", Decode, DecodeConfig)
}
