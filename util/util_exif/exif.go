package util_exif

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/storage/datastore"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type ExifOrientation struct {
	RotateDegrees  int // should be 0, 90, 180, or 270
	FlipVertical   bool
	FlipHorizontal bool
}

func GetExifOrientation(media *types.Media) (*ExifOrientation, error) {
	if media.ContentType != "image/jpeg" && media.ContentType != "image/jpg" {
		return nil, errors.New("image is not a jpeg")
	}

	mediaStream, err := datastore.DownloadStream(rcontext.Initial(), media.DatastoreId, media.Location)
	if err != nil {
		return nil, err
	}
	defer util.DumpAndCloseStream(mediaStream)

	exifData, err := exif.Decode(mediaStream)
	if err != nil {
		return nil, err
	}

	rawValue, err := exifData.Get(exif.Orientation)
	if err != nil {
		return nil, err
	}

	orientation, err := rawValue.Int(0)
	if err != nil {
		return nil, err
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
	} else if orientation == 7 || degrees == 8 {
		degrees = 90
	}

	return &ExifOrientation{degrees, flipVertical, flipHorizontal}, nil
}
