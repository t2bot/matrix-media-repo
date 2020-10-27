package u

import (
	"bytes"
	"errors"
	"image"
	"io/ioutil"

	"github.com/disintegration/imaging"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/util/util_exif"
)

func MakeThumbnail(src image.Image, method string, width int, height int) (image.Image, error) {
	var result image.Image
	if method == "scale" {
		result = imaging.Fit(src, width, height, imaging.Linear)
	} else if method == "crop" {
		result = imaging.Fill(src, width, height, imaging.Center, imaging.Linear)
	} else {
		return nil, errors.New("unrecognized method: " + method)
	}
	return result, nil
}

func IdentifyAndApplyOrientation(origBytes []byte, src image.Image) (image.Image, error) {
	orientation, err := util_exif.GetExifOrientation(ioutil.NopCloser(bytes.NewBuffer(origBytes)))
	if err != nil {
		// assume no orientation if there was an error reading the exif header
		logrus.Warn("Non-fatal error reading exif headers:", err.Error())
		orientation = nil
	}

	result := src
	if orientation != nil {
		// Rotate first
		if orientation.RotateDegrees == 90 {
			result = imaging.Rotate90(result)
		} else if orientation.RotateDegrees == 180 {
			result = imaging.Rotate180(result)
		} else if orientation.RotateDegrees == 270 {
			result = imaging.Rotate270(result)
		} // else we don't care to rotate

		// Flip second
		if orientation.FlipHorizontal {
			result = imaging.FlipH(result)
		}
		if orientation.FlipVertical {
			result = imaging.FlipV(result)
		}
	}

	return result, nil
}
