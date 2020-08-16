package i

import (
	"errors"

	"github.com/faiface/beep/flac"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/thumbnailing/m"
	"github.com/turt2live/matrix-media-repo/util"
)

type flacGenerator struct {
}

func (d flacGenerator) supportedContentTypes() []string {
	return []string{"audio/flac"}
}

func (d flacGenerator) supportsAnimation() bool {
	return false
}

func (d flacGenerator) matches(img []byte, contentType string) bool {
	return contentType == "audio/flac"
}

func (d flacGenerator) GenerateThumbnail(b []byte, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	audio, format, err := flac.Decode(util.ByteCloser(b))
	if err != nil {
		return nil, errors.New("flac: error decoding audio: " + err.Error())
	}

	defer audio.Close()
	return mp3Generator{}.GenerateFromStream(audio, format, width, height)
}

func init() {
	generators = append(generators, flacGenerator{})
}
