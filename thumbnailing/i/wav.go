package i

import (
	"errors"

	"github.com/faiface/beep/wav"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/thumbnailing/m"
	"github.com/turt2live/matrix-media-repo/util"
)

type wavGenerator struct {
}

func (d wavGenerator) supportedContentTypes() []string {
	return []string{"audio/wav"}
}

func (d wavGenerator) supportsAnimation() bool {
	return false
}

func (d wavGenerator) matches(img []byte, contentType string) bool {
	return contentType == "audio/wav"
}

func (d wavGenerator) GenerateThumbnail(b []byte, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	audio, format, err := wav.Decode(util.ByteCloser(b))
	if err != nil {
		return nil, errors.New("wav: error decoding audio: " + err.Error())
	}

	defer audio.Close()
	return mp3Generator{}.GenerateFromStream(audio, format, width, height)
}

func init() {
	generators = append(generators, wavGenerator{})
}
