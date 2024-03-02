package preview

import (
	"fmt"
	"io"

	"github.com/faiface/beep"
	"github.com/faiface/beep/vorbis"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/thumbnailing/u"
	"github.com/t2bot/matrix-media-repo/util/readers"
)

type oggGenerator struct{}

func (d oggGenerator) supportedContentTypes() []string {
	return []string{"audio/ogg"}
}

func (d oggGenerator) supportsAnimation() bool {
	return false
}

func (d oggGenerator) matches(img io.Reader, contentType string) bool {
	return contentType == "audio/ogg"
}

func (d oggGenerator) decode(b io.Reader) (beep.StreamSeekCloser, beep.Format, error) {
	audio, format, err := vorbis.Decode(readers.MakeCloser(b))
	if err != nil {
		return audio, format, fmt.Errorf("ogg: error decoding audio: %w", err)
	}
	return audio, format, nil
}

func (d oggGenerator) GetOriginDimensions(b io.Reader, contentType string, ctx rcontext.RequestContext) (bool, int, int, error) {
	return false, 0, 0, nil
}

func (d oggGenerator) GenerateThumbnail(b io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*Thumbnail, error) {
	tags, rc, err := u.GetID3Tags(b)
	if err != nil {
		return nil, fmt.Errorf("ogg: error getting tags: %v", err)
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

func (d oggGenerator) GetAudioData(b io.Reader, nKeys int, ctx rcontext.RequestContext) (*AudioInfo, error) {
	audio, format, err := d.decode(b)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer audio.Close()
	return mp3Generator{}.GetDataFromStream(audio, format, nKeys)
}

func init() {
	generators = append(generators, oggGenerator{})
}
