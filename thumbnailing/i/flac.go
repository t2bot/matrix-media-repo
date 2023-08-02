package i

import (
	"errors"
	"io"

	"github.com/faiface/beep"
	"github.com/faiface/beep/flac"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/thumbnailing/m"
	"github.com/turt2live/matrix-media-repo/thumbnailing/u"
)

type flacGenerator struct {
}

func (d flacGenerator) supportedContentTypes() []string {
	return []string{"audio/flac"}
}

func (d flacGenerator) supportsAnimation() bool {
	return false
}

func (d flacGenerator) matches(img io.Reader, contentType string) bool {
	return contentType == "audio/flac"
}

func (d flacGenerator) decode(b io.Reader) (beep.StreamSeekCloser, beep.Format, error) {
	audio, format, err := flac.Decode(b)
	if err != nil {
		return audio, format, errors.New("flac: error decoding audio: " + err.Error())
	}
	return audio, format, nil
}

func (d flacGenerator) GetOriginDimensions(b io.Reader, contentType string, ctx rcontext.RequestContext) (bool, int, int, error) {
	return false, 0, 0, nil
}

func (d flacGenerator) GenerateThumbnail(r io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	tags, rc, err := u.GetID3Tags(r)
	if err != nil {
		return nil, errors.New("flac: error getting tags: " + err.Error())
	}
	//goland:noinspection GoUnhandledErrorResult
	defer rc.Close()

	audio, format, err := d.decode(rc)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer audio.Close()
	return mp3Generator{}.GenerateFromStream(audio, format, tags, width, height, ctx)
}

func (d flacGenerator) GetAudioData(b io.Reader, nKeys int, ctx rcontext.RequestContext) (*m.AudioInfo, error) {
	audio, format, err := d.decode(b)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer audio.Close()
	return mp3Generator{}.GetDataFromStream(audio, format, nKeys)
}

func init() {
	generators = append(generators, flacGenerator{})
}
