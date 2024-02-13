package thumbnailing

import (
	"errors"
	"io"
	"reflect"

	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/thumbnailing/i"
	"github.com/t2bot/matrix-media-repo/thumbnailing/m"
	"github.com/t2bot/matrix-media-repo/thumbnailing/u"
	"github.com/t2bot/matrix-media-repo/util"
	"github.com/t2bot/matrix-media-repo/util/readers"
)

var ErrUnsupported = errors.New("unsupported thumbnail type")

func IsSupported(contentType string) bool {
	return util.ArrayContains(i.GetSupportedContentTypes(), contentType)
}

func GenerateThumbnail(imgStream io.ReadCloser, contentType string, width int, height int, method string, animated bool, ctx rcontext.RequestContext) (*m.Thumbnail, error) {
	defer imgStream.Close()
	if !IsSupported(contentType) {
		ctx.Log.Debugf("Unsupported content type '%s'", contentType)
		return nil, ErrUnsupported
	}
	if !util.ArrayContains(ctx.Config.Thumbnails.Types, contentType) {
		ctx.Log.Debugf("Disabled content type '%s'", contentType)
		return nil, ErrUnsupported
	}

	generator, reconstructed := i.GetGenerator(imgStream, contentType, animated)
	if generator == nil {
		ctx.Log.Debugf("Unsupported thumbnail type at generator for '%s'", contentType)
		return nil, ErrUnsupported
	}
	ctx.Log.Debug("Using generator: ", reflect.TypeOf(generator).Name())

	// Validate maximum megapixel values to avoid memory issues
	// https://github.com/t2bot/matrix-media-repo/security/advisories/GHSA-j889-h476-hh9h
	buffered := readers.NewBufferReadsReader(reconstructed)
	dimensional, w, h, err := generator.GetOriginDimensions(buffered, contentType, ctx)
	if err != nil {
		return nil, errors.New("error getting dimensions: " + err.Error())
	}
	if dimensional {
		if (w * h) >= ctx.Config.Thumbnails.MaxPixels {
			ctx.Log.Debug("Image too large: too many pixels")
			return nil, common.ErrMediaTooLarge
		}

		// While we're here, check to ensure we're not about to produce a thumbnail which is larger than the source material
		shouldThumbnail := true
		shouldThumbnail, width, height, method = u.AdjustProperties(w, h, width, height, animated, method)
		if !shouldThumbnail {
			return nil, common.ErrMediaDimensionsTooSmall
		}
	}

	return generator.GenerateThumbnail(buffered.GetRewoundReader(), contentType, width, height, method, animated, ctx)
}

func GetGenerator(imgStream io.Reader, contentType string, animated bool) (i.Generator, io.Reader, error) {
	generator, reconstructed := i.GetGenerator(imgStream, contentType, animated)
	if generator == nil {
		return nil, reconstructed, ErrUnsupported
	}

	return generator, reconstructed, nil
}
