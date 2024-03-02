package preview

import (
	"io"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/thumbnailing/m"
	"github.com/t2bot/matrix-media-repo/util/readers"
)

type Generator interface {
	supportedContentTypes() []string
	supportsAnimation() bool
	matches(img io.Reader, contentType string) bool
	GenerateThumbnail(img io.Reader, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error)
	GetOriginDimensions(b io.Reader, contentType string, ctx rcontext.RequestContext) (bool, int, int, error)
}

type AudioGenerator interface {
	Generator
	GetAudioData(b io.Reader, nKeys int, ctx rcontext.RequestContext) (*m.AudioInfo, error)
}

var generators = make([]Generator, 0)

func GetGenerator(img io.Reader, contentType string, needsAnimation bool) (Generator, io.Reader) {
	br := readers.NewBufferReadsReader(img)
	for _, g := range generators {
		if needsAnimation && !g.supportsAnimation() {
			continue
		}
		if g.matches(br, contentType) {
			return g, br.GetRewoundReader()
		}
	}
	if needsAnimation {
		// try again, this time without animation
		return GetGenerator(br.GetRewoundReader(), contentType, false)
	}
	return nil, br.GetRewoundReader()
}

func GetSupportedContentTypes() []string {
	a := make([]string, 0)
	for _, d := range generators {
		a = append(a, d.supportedContentTypes()...)
	}
	return a
}
