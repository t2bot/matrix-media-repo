package thumbnailing

import (
	"errors"
	"io"
	"reflect"

	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/thumbnailing/i"
	"github.com/turt2live/matrix-media-repo/thumbnailing/m"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/readers"
)

var ErrUnsupported = errors.New("unsupported thumbnail type")

func IsSupported(contentType string) bool {
	return util.ArrayContains(i.GetSupportedContentTypes(), contentType)
}

func IsAnimationSupported(contentType string) bool {
	return util.ArrayContains(i.GetSupportedAnimationTypes(), contentType)
}

func GenerateThumbnail(imgStream io.ReadCloser, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	defer imgStream.Close()
	if !IsSupported(contentType) {
		return nil, ErrUnsupported
	}

	generator, reconstructed := i.GetGenerator(imgStream, contentType, animated)
	if generator == nil {
		return nil, ErrUnsupported
	}
	ctx.Log.Info("Using generator: ", reflect.TypeOf(generator).Name())

	// Validate maximum megapixel values to avoid memory issues
	// https://github.com/turt2live/matrix-media-repo/security/advisories/GHSA-j889-h476-hh9h
	buffered := readers.NewBufferReadsReader(reconstructed)
	dimensional, w, h, err := generator.GetOriginDimensions(buffered, contentType, ctx)
	if err != nil {
		return nil, errors.New("error getting dimensions: " + err.Error())
	}
	if dimensional && (w*h) >= ctx.Config.Thumbnails.MaxPixels {
		ctx.Log.Debug("Image too large: too many pixels")
		return nil, common.ErrMediaTooLarge
	}

	return generator.GenerateThumbnail(buffered.GetRewoundReader(), contentType, width, height, method, animated, ctx)
}

func GetGenerator(imgStream io.Reader, contentType string, animated bool) (i.Generator, *readers.PrefixedReader, error) {
	generator, reconstructed := i.GetGenerator(imgStream, contentType, animated)
	if generator == nil {
		return nil, reconstructed, ErrUnsupported
	}

	return generator, reconstructed, nil
}
