package i

import (
	"errors"

	"github.com/faiface/beep"
	"github.com/faiface/beep/wav"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/thumbnailing/m"
	"github.com/turt2live/matrix-media-repo/thumbnailing/u"
	"github.com/turt2live/matrix-media-repo/util/util_byte_seeker"
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

func (d wavGenerator) decode(b []byte) (beep.StreamSeekCloser, beep.Format, error) {
	audio, format, err := wav.Decode(util_byte_seeker.NewByteSeeker(b))
	if err != nil {
		return audio, format, errors.New("wav: error decoding audio: " + err.Error())
	}
	return audio, format, nil
}

func (d wavGenerator) GenerateThumbnail(b []byte, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	audio, format, err := d.decode(b)
	if err != nil {
		return nil, err
	}

	defer audio.Close()
	return mp3Generator{}.GenerateFromStream(audio, format, u.GetID3Tags(b), width, height)
}

func (d wavGenerator) GetAudioData(b []byte, nKeys int, ctx rcontext.RequestContext) (*m.AudioInfo, error) {
	audio, format, err := d.decode(b)
	if err != nil {
		return nil, err
	}

	defer audio.Close()
	return mp3Generator{}.GetDataFromStream(audio, format, nKeys)
}

func init() {
	generators = append(generators, wavGenerator{})
}
