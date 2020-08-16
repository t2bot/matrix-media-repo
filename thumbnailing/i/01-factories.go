package i

import (
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/thumbnailing/m"
)

type Generator interface {
	supportedContentTypes() []string
	supportsAnimation() bool
	matches(img []byte, contentType string) bool
	GenerateThumbnail(b []byte, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error)
}

type AudioGenerator interface {
	GetAudioData(b []byte, nKeys int, ctx rcontext.RequestContext) (*m.AudioInfo, error)
}

var generators = make([]Generator, 0)

func GetGenerator(img []byte, contentType string, needsAnimation bool) Generator {
	for _, g := range generators {
		if needsAnimation && !g.supportsAnimation() {
			continue
		}
		if g.matches(img, contentType) {
			return g
		}
	}
	if needsAnimation {
		// try again, this time without animation
		return GetGenerator(img, contentType, false)
	}
	return nil
}

func GetSupportedContentTypes() []string {
	a := make([]string, 0)
	for _, d := range generators {
		for _, c := range d.supportedContentTypes() {
			a = append(a, c)
		}
	}
	return a
}

func GetSupportedAnimationTypes() []string {
	a := make([]string, 0)
	for _, d := range generators {
		if !d.supportsAnimation() {
			continue
		}
		for _, c := range d.supportedContentTypes() {
			a = append(a, c)
		}
	}
	return a
}
