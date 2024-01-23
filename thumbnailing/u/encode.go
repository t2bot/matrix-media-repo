package u

import (
	"image"
	"io"

	"github.com/disintegration/imaging"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

type EncodeSource int

const (
	GenericSource EncodeSource = 0
	JpegSource    EncodeSource = 1
)

func Encode(ctx rcontext.RequestContext, w io.Writer, img image.Image, sourceFlags ...EncodeSource) error {
	// This function is broken out for later trials around encoding formats (webp, jpg, etc)

	if len(sourceFlags) > 0 {
		for _, f := range sourceFlags {
			if f == JpegSource {
				// Encode JPEG source with JPEG thumbnails to avoid returning larger thumbnails
				// than what we started with
				return imaging.Encode(w, img, imaging.JPEG)
			}
		}
	}

	return imaging.Encode(w, img, imaging.PNG)
}
