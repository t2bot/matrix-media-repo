package thumbnailing

import (
	"errors"
	"io"
	"reflect"

	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/thumbnailing/i"
	"github.com/turt2live/matrix-media-repo/thumbnailing/m"
	"github.com/turt2live/matrix-media-repo/thumbnailing/u"
	"github.com/turt2live/matrix-media-repo/util"
	"github.com/turt2live/matrix-media-repo/util/readers"
)

var ErrUnsupported = errors.New("unsupported thumbnail type")

func IsSupported(contentType string) bool {
	return util.ArrayContains(i.GetSupportedContentTypes(), contentType)
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
	ctx.Log.Debug("Using generator: ", reflect.TypeOf(generator).Name())

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

	// TODO: Why does AdjustProperties even take `canAnimate` if it's always been hardcoded to `false`? (see git blame on this comment)
	var shouldThumbnail bool
	shouldThumbnail, width, height, _, method = u.AdjustProperties(w, h, width, height, animated, false, method)
	if !shouldThumbnail {
		return nil, common.ErrMediaDimensionsTooSmall
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
