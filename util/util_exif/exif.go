package util_exif

import (
	"fmt"
	"io"

	"github.com/pkg/errors"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
)

type ExifOrientation struct {
	RotateDegrees  int // should be 0, 90, 180, or 270
	FlipVertical   bool
	FlipHorizontal bool
}

func GetExifOrientation(img io.ReadCloser) (*ExifOrientation, error) {
	defer cleanup.DumpAndCloseStream(img)

	exifData, err := exif.Decode(img)
	if err != nil {
		// EOF means we probably just don't have info in the file
		if err == io.EOF {
			return nil, nil
		}
		return nil, errors.New("exif: error decoding orientation: " + err.Error())
	}

	rawValue, err := exifData.Get(exif.Orientation)
	if exif.IsTagNotPresentError(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("exif: error getting orientation: " + err.Error())
	}

	orientation, err := rawValue.Int(0)
	if err != nil {
		return nil, errors.New("exif: error parsing orientation: " + err.Error())
	}

	// Some devices produce invalid exif data when they intend to mean "no orientation"
	if orientation == 0 {
		return nil, nil
	}

	if orientation < 1 || orientation > 8 {
		return nil, errors.New(fmt.Sprintf("orientation out of range: %d", orientation))
	}

	flipHorizontal := orientation < 5 && (orientation%2) == 0
	flipVertical := orientation > 4 && (orientation%2) != 0
	degrees := 0

	// TODO: There's probably a better way to represent this
	if orientation == 1 || orientation == 2 {
		degrees = 0
	} else if orientation == 3 || orientation == 4 {
		degrees = 180
	} else if orientation == 5 || orientation == 6 {
		degrees = 270
	} else if orientation == 7 || orientation == 8 {
		degrees = 90
	}

	return &ExifOrientation{degrees, flipVertical, flipHorizontal}, nil
}
