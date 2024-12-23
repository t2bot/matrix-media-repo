package u

import (
	"errors"
	"image"
	"io"

	"github.com/davidbyttow/govips/v2/vips"
	"github.com/disintegration/imaging"
	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
)

/*
func MakeThumbnail(src image.Image, method string, width int, height int) (image.Image, error) {
	buff := new(bytes.Buffer)
	err := png.Encode(buff, src)
	if err != nil {
		return nil, errors.New("thumbnail: error when preprocessing the file: " + err.Error())
	}
	i, err := vips.NewImageFromBuffer(buff.Bytes())
	if err != nil {
		return nil, errors.New("thumbnail: error when loading by vips: " + err.Error())
	}
	tByte, err := MakeThumbnailByVips(i, method, width, height)
	if err != nil {
		return nil, err
	}
	result, _, err := image.Decode(bytes.NewReader(tByte))
	if err != nil {
		return nil, errors.New("webp: error decoding thumbnail: " + err.Error())
	}
	return result, nil
}
*/

func MakeThumbnailByImaging(src image.Image, method string, width int, height int) (image.Image, error) {
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

func MakeThumbnailByVips(i *vips.ImageRef, method string, width int, height int, animated bool) ([]byte, error) {
	var result []byte
	var err error
	if method == "scale" {
		err = i.Thumbnail(width, height, vips.InterestingNone)
	} else if method == "crop" {
		if animated {
			err = i.SmartCrop(width, height, vips.InterestingNone)
		} else {
			err = i.SmartCrop(width, height, vips.InterestingCentre)
		}
	} else {
		return nil, errors.New("unrecognized method: " + method)
	}
	if err != nil {
		return nil, err
	}
	result, _, err = i.ExportNative()
	return result, err
}

func ExtractExifOrientation(r io.Reader) *ExifOrientation {
	orientation, err := GetExifOrientation(r)
	if err != nil {
		// assume no orientation if there was an error reading the exif header
		logrus.Warn("Non-fatal error reading exif headers:", err.Error())
		sentry.CaptureException(err)
		orientation = nil
	}
	return orientation
}

func ApplyOrientationByImaging(src image.Image, orientation *ExifOrientation) image.Image {
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

	return result
}

func ApplyOrientationByVips(i *vips.ImageRef, orientation *ExifOrientation) *vips.ImageRef {
	if orientation != nil {
		// Rotate first
		if orientation.RotateDegrees == 90 {
			i.Rotate(vips.Angle90)
		} else if orientation.RotateDegrees == 180 {
			i.Rotate(vips.Angle180)
		} else if orientation.RotateDegrees == 270 {
			i.Rotate(vips.Angle270)
		}

		// Flip second
		if orientation.FlipHorizontal {
			i.Flip(vips.DirectionHorizontal)
		}
		if orientation.FlipVertical {
			i.Flip(vips.DirectionVertical)
		}
	}

	return i
}
