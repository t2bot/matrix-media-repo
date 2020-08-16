package thumbnailing

import (
	"errors"
	"io"
	"io/ioutil"
	"reflect"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/thumbnailing/i"
	"github.com/turt2live/matrix-media-repo/thumbnailing/m"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
)

var ErrUnsupported = errors.New("unsupported thumbnail type")

func IsSupported(contentType string) bool {
	return util.ArrayContains(i.GetSupportedContentTypes(), contentType)
}

func IsAnimationSupported(contentType string) bool {
	return util.ArrayContains(i.GetSupportedAnimationTypes(), contentType)
}

func GenerateThumbnail(imgStream io.ReadCloser, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	if !IsSupported(contentType) {
		return nil, ErrUnsupported
	}

	defer cleanup.DumpAndCloseStream(imgStream)
	b, err := ioutil.ReadAll(imgStream)
	if err != nil {
		return nil, err
	}

	generator := i.GetGenerator(b, contentType, animated)
	if generator == nil {
		return nil, ErrUnsupported
	}
	ctx.Log.Info("Using generator: ", reflect.TypeOf(generator).Name())

	return generator.GenerateThumbnail(b, contentType, width, height, method, animated, ctx)
}

func GetGenerator(imgStream io.ReadCloser, contentType string, animated bool) (i.Generator, error) {
	defer cleanup.DumpAndCloseStream(imgStream)
	b, err := ioutil.ReadAll(imgStream)
	if err != nil {
		return nil, err
	}

	generator := i.GetGenerator(b, contentType, animated)
	if generator == nil {
		return nil, ErrUnsupported
	}

	return generator, nil
}
