package thumbnails

import (
	"errors"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/util"
)

func PickNewDimensions(ctx rcontext.RequestContext, desiredWidth int, desiredHeight int, desiredMethod string) (int, int, string, error) {
	if desiredWidth <= 0 {
		return 0, 0, "", errors.New("width must be positive")
	}
	if desiredHeight <= 0 {
		return 0, 0, "", errors.New("height must be positive")
	}
	if desiredMethod != "crop" && desiredMethod != "scale" {
		return 0, 0, "", errors.New("method must be crop or scale")
	}

	foundSize := false
	targetWidth := 0
	targetHeight := 0
	largestWidth := 0
	largestHeight := 0
	desiredAspectRatio := float32(desiredWidth) / float32(desiredHeight)

	for _, size := range ctx.Config.Thumbnails.Sizes {
		largestWidth = util.MaxInt(largestWidth, size.Width)
		largestHeight = util.MaxInt(largestHeight, size.Height)

		// Unlikely, but if we get the exact dimensions then just use that
		if desiredWidth == size.Width && desiredHeight == size.Height {
			return size.Width, size.Height, desiredMethod, nil
		}

		// If we come across a size that's larger than requested, try and use that
		if desiredWidth <= size.Width && desiredHeight <= size.Height {
			// Only use our new found size if it's smaller than one we've already picked
			if !foundSize || (targetWidth > size.Width && targetHeight > size.Height) {
				targetWidth = size.Width
				targetHeight = size.Height
				foundSize = true
			}
		}
	}

	if ctx.Config.Thumbnails.DynamicSizing {
		return util.MinInt(largestWidth, desiredWidth), util.MinInt(largestHeight, desiredHeight), desiredMethod, nil
	}

	// Use the largest dimensions available if we didn't find anything
	if !foundSize {
		targetWidth = largestWidth
		targetHeight = largestHeight
	}

	if desiredMethod == "crop" {
		// We need to maintain the aspect ratio of the request
		sizeAspect := float32(targetWidth) / float32(targetHeight)
		if sizeAspect != desiredAspectRatio { // it's unlikely to match, but we can dream
			ratio := util.MinFloat32(float32(targetWidth)/float32(desiredWidth), float32(targetHeight)/float32(desiredHeight))
			targetWidth = int(float32(desiredWidth) * ratio)
			targetHeight = int(float32(desiredHeight) * ratio)
		}
	}

	return targetWidth, targetHeight, desiredMethod, nil
}
