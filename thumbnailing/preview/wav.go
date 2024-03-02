package preview

import (
	"fmt"
	"io"

	"github.com/dhowden/tag"
	"github.com/faiface/beep"
	"github.com/faiface/beep/wav"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

type wavGenerator struct{}

func (d wavGenerator) supportedContentTypes() []string {
	return []string{"audio/wav"}
}

func (d wavGenerator) supportsAnimation() bool {
	return false
}

func (d wavGenerator) matches(img io.Reader, contentType string) bool {
	return contentType == "audio/wav"
}

func (d wavGenerator) decode(b io.Reader) (beep.StreamSeekCloser, beep.Format, error) {
	audio, format, err := wav.Decode(b)
	if err != nil {
		return audio, format, fmt.Errorf("wav: error decoding audio: %w", err)
	}
	return audio, format, nil
}

func (d wavGenerator) GetOriginDimensions(b io.Reader, contentType string, ctx rcontext.RequestContext) (bool, int, int, error) {
	return false, 0, 0, nil
}

func (d wavGenerator) GenerateThumbnail(b io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*Thumbnail, error) {
	rd, err := newReadSeekerWrapper(b)
	tags, err := tag.ReadFrom(rd) // we don't care about errors in this process
	if err != nil {
		return nil, fmt.Errorf("wav: error getting tags: %w", err)
	}

	audio, format, err := d.decode(rd)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer audio.Close()
	return mp3Generator{}.GenerateFromStream(audio, format, tags, width, height, ctx)
}

func (d wavGenerator) GetAudioData(b io.Reader, nKeys int, ctx rcontext.RequestContext) (*AudioInfo, error) {
	audio, format, err := d.decode(b)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer audio.Close()
	return mp3Generator{}.GetDataFromStream(audio, format, nKeys)
}

func init() {
	generators = append(generators, wavGenerator{})
}
