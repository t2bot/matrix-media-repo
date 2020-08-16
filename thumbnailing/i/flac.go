package i

import (
	"errors"

	"github.com/faiface/beep"
	"github.com/faiface/beep/flac"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/thumbnailing/m"
	"github.com/turt2live/matrix-media-repo/thumbnailing/u"
	"github.com/turt2live/matrix-media-repo/util/util_byte_seeker"
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

func (d flacGenerator) decode(b []byte) (beep.StreamSeekCloser, beep.Format, error) {
	audio, format, err := flac.Decode(util_byte_seeker.NewByteSeeker(b))
	if err != nil {
		return audio, format, errors.New("flac: error decoding audio: " + err.Error())
	}
	return audio, format, nil
}

func (d flacGenerator) GenerateThumbnail(b []byte, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	audio, format, err := d.decode(b)
	if err != nil {
		return nil, err
	}

	defer audio.Close()
	return mp3Generator{}.GenerateFromStream(audio, format, u.GetID3Tags(b), width, height)
}

func (d flacGenerator) GetAudioData(b []byte, nKeys int, ctx rcontext.RequestContext) (*m.AudioInfo, error) {
	audio, format, err := d.decode(b)
	if err != nil {
		return nil, err
	}

	defer audio.Close()
	return mp3Generator{}.GetDataFromStream(audio, format, nKeys)
}

func init() {
	generators = append(generators, flacGenerator{})
}
