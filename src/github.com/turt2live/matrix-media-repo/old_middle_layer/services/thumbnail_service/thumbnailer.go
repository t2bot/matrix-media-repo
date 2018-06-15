package thumbnail_service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/draw"
	"image/gif"
	"os"

	"github.com/disintegration/imaging"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type generatedThumbnail struct {
	ContentType  string
	DiskLocation string
	SizeBytes    int64
	Animated     bool
	Sha256Hash   *string
}

type thumbnailer struct {
	ctx context.Context
	log *logrus.Entry
}

func newThumbnailer(ctx context.Context, log *logrus.Entry) *thumbnailer {
	return &thumbnailer{ctx, log}
}

func (t *thumbnailer) GenerateThumbnail(media *types.Media, width int, height int, method string, animated bool, forceGeneration bool) (*generatedThumbnail, error) {
	if animated && !util.ArrayContains(AnimatedTypes, media.ContentType) {
		t.log.Warn("Attempted to animate a media record that isn't an animated type. Assuming animated=false")
		animated = false
	}

	var src image.Image
	var err error

	if media.ContentType == "image/svg+xml" {
		src, err = t.svgToImage(media)
	} else {
		src, err = imaging.Open(media.Location)
	}

	if err != nil {
		return nil, err
	}

	srcWidth := src.Bounds().Max.X
	srcHeight := src.Bounds().Max.Y

	aspectRatio := float32(srcHeight) / float32(srcWidth)
	targetAspectRatio := float32(width) / float32(height)
	if aspectRatio == targetAspectRatio {
		// Highly unlikely, but if the aspect ratios match then just resize
		method = "scale"
		t.log.Info("Aspect ratio is the same, converting method to 'scale'")
	}

	thumb := &generatedThumbnail{
		Animated: animated,
	}

	if srcWidth <= width && srcHeight <= height {
		if forceGeneration {
			t.log.Warn("Image is too small but the force flag is set. Adjusting dimensions to fit image exactly.")
			width = srcWidth
			height = srcHeight
		} else {
			// Image is too small - don't upscale
			thumb.ContentType = media.ContentType
			thumb.DiskLocation = media.Location
			thumb.SizeBytes = media.SizeBytes
			thumb.Sha256Hash = &media.Sha256Hash
			t.log.Warn("Image too small, returning raw image")
			return thumb, nil
		}
	}

	var orientation *util.ExifOrientation = nil
	if media.ContentType == "image/jpeg" || media.ContentType == "image/jpg" {
		orientation, err = util.GetExifOrientation(media)
		if err != nil {
			t.log.Warn("Non-fatal error getting EXIF orientation: " + err.Error())
			orientation = nil // just in case
		}
	}

	contentType := "image/png"
	imgData := &bytes.Buffer{}
	if config.Get().Thumbnails.AllowAnimated && animated && util.ArrayContains(AnimatedTypes, media.ContentType) {
		t.log.Info("Generating animated thumbnail")
		contentType = "image/gif"

		// Animated GIFs are a bit more special because we need to do it frame by frame.
		// This is fairly resource intensive. The calling code is responsible for limiting this case.

		inputFile, err := os.Open(media.Location)
		if err != nil {
			t.log.Error("Error generating animated thumbnail: " + err.Error())
			return nil, err
		}
		defer inputFile.Close()

		g, err := gif.DecodeAll(inputFile)
		if err != nil {
			t.log.Error("Error generating animated thumbnail: " + err.Error())
			return nil, err
		}

		// Prepare a blank frame to use as swap space
		frameImg := image.NewRGBA(g.Image[0].Bounds())

		for i := range g.Image {
			img := g.Image[i]

			// Clear the transparency of the previous frame
			draw.Draw(frameImg, frameImg.Bounds(), image.Transparent, image.ZP, draw.Src)

			// Copy the frame to a new image and use that
			draw.Draw(frameImg, frameImg.Bounds(), img, image.ZP, draw.Over)

			// Do the thumbnailing on the copied frame
			frameThumb, err := thumbnailFrame(frameImg, method, width, height, imaging.Linear, nil)
			if err != nil {
				t.log.Error("Error generating animated thumbnail frame: " + err.Error())
				return nil, err
			}

			//t.log.Info(fmt.Sprintf("Width = %d    Height = %d    FW=%d    FH=%d", width, height, frameThumb.Bounds().Max.X, frameThumb.Bounds().Max.Y))

			targetImg := image.NewPaletted(frameThumb.Bounds(), img.Palette)
			draw.FloydSteinberg.Draw(targetImg, frameThumb.Bounds(), frameThumb, image.ZP)
			g.Image[i] = targetImg
		}

		// Set the image size to the first frame's size
		g.Config.Width = g.Image[0].Bounds().Max.X
		g.Config.Height = g.Image[0].Bounds().Max.Y

		err = gif.EncodeAll(imgData, g)
		if err != nil {
			t.log.Error("Error generating animated thumbnail: " + err.Error())
			return nil, err
		}
	} else {
		src, err = thumbnailFrame(src, method, width, height, imaging.Lanczos, orientation)
		if err != nil {
			t.log.Error("Error generating thumbnail: " + err.Error())
			return nil, err
		}

		// Put the image bytes into a memory buffer
		err = imaging.Encode(imgData, src, imaging.PNG)
		if err != nil {
			t.log.Error("Unexpected error encoding thumbnail: " + err.Error())
			return nil, err
		}
	}

	// Reset the buffer pointer and store the file
	location, err := storage.PersistFile(imgData, t.ctx, t.log)
	if err != nil {
		t.log.Error("Unexpected error saving thumbnail: " + err.Error())
		return nil, err
	}

	fileSize, err := util.FileSize(location)
	if err != nil {
		t.log.Error("Unexpected error getting the size of the thumbnail: " + err.Error())
		return nil, err
	}

	hash, err := storage.GetFileHash(location)
	if err != nil {
		t.log.Error("Unexpected error getting the hash for the thumbnail: ", err.Error())
		return nil, err
	}

	thumb.DiskLocation = location
	thumb.ContentType = contentType
	thumb.SizeBytes = fileSize
	thumb.Sha256Hash = &hash

	return thumb, nil
}

func thumbnailFrame(src image.Image, method string, width int, height int, filter imaging.ResampleFilter, orientation *util.ExifOrientation) (image.Image, error) {
	var result image.Image
	if method == "scale" {
		result = imaging.Fit(src, width, height, filter)
	} else if method == "crop" {
		result = imaging.Fill(src, width, height, imaging.Center, filter)
	} else {
		return nil, errors.New("unrecognized method: " + method)
	}

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
