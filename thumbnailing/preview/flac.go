package preview

import (
	"fmt"
	"io"

	"github.com/dhowden/tag"
	"github.com/faiface/beep"
	"github.com/faiface/beep/flac"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

type flacGenerator struct{}

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
		return audio, format, fmt.Errorf("flac: error decoding audio: %w", err)
	}
	return audio, format, nil
}

func (d flacGenerator) GetOriginDimensions(b io.Reader, contentType string, ctx rcontext.RequestContext) (bool, int, int, error) {
	return false, 0, 0, nil
}

func (d flacGenerator) GenerateThumbnail(r io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*Thumbnail, error) {
	rd, err := newReadSeekerWrapper(r)
	if err != nil {
		return nil, fmt.Errorf("error wrapping reader: %w", err)
	}
	tags, err := tag.ReadFrom(rd)
	if err != nil {
		return nil, fmt.Errorf("flac: error getting tags: %w", err)
	}

	audio, format, err := d.decode(rd)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer audio.Close()
	return mp3Generator{}.GenerateFromStream(audio, format, tags, width, height, ctx)
}

func (d flacGenerator) GetAudioData(b io.Reader, nKeys int, ctx rcontext.RequestContext) (*AudioInfo, error) {
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
