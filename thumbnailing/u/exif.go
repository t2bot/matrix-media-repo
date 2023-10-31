package u

import (
	"errors"
	"fmt"
	"io"

	"github.com/dsoprea/go-exif/v3"
)

type ExifOrientation struct {
	RotateDegrees  int // should be 0, 90, 180, or 270
	FlipVertical   bool
	FlipHorizontal bool
}

func GetExifOrientation(img io.Reader) (*ExifOrientation, error) {
	rawExif, err := exif.SearchAndExtractExifWithReader(img)
	if err != nil {
		if errors.Is(err, exif.ErrNoExif) {
			return nil, nil
		}
		return nil, errors.New("exif: error reading possible exif data: " + err.Error())
	}

	tags, _, err := exif.GetFlatExifData(rawExif, nil)
	if err != nil {
		return nil, errors.New("exif: error parsing exif data: " + err.Error())
	}

	var tag exif.ExifTag
	for _, t := range tags {
		if t.TagName == "Orientation" {
			tag = t
			break
		}
	}
	if tag.TagName != "Orientation" {
		return nil, nil // not found
	}

	var orientation uint16 = 0
	vals, ok := tag.Value.([]uint16)
	if !ok || len(vals) <= 0 {
		orientation, ok = tag.Value.(uint16)
		if !ok {
			return nil, errors.New("exif: error parsing orientation: parse error (not an int)")
		}
	} else {
		orientation = vals[0]
	}

	// Some devices produce invalid exif data when they intend to mean "no orientation"
	if orientation == 0 {
		return nil, nil
	}

	if orientation < 1 || orientation > 8 {
		return nil, fmt.Errorf("orientation out of range: %d", orientation)
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
