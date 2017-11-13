package media_handler

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/disintegration/imaging"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/config"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

func GetThumbnail(ctx context.Context, media types.Media, width int, height int, method string, c config.MediaRepoConfig, db storage.Database, log *logrus.Entry) (types.Thumbnail, error) {
	if width <= 0 {
		return types.Thumbnail{}, errors.New("width must be positive")
	}
	if height <= 0 {
		return types.Thumbnail{}, errors.New("height must be positive")
	}
	if method != "crop" && method != "scale" {
		return types.Thumbnail{}, errors.New("method must be crop or scale")
	}

	targetWidth := width
	targetHeight := height
	foundFirst := false

	for i := 0; i < len(c.Thumbnails.Sizes); i++ {
		size := c.Thumbnails.Sizes[i]
		lastSize := i == len(c.Thumbnails.Sizes) - 1

		if width == size.Width && height == size.Height {
			targetWidth = width
			targetHeight = height
			break
		}

		if (size.Width < width || size.Height < height) && !lastSize {
			continue // too small
		}

		diffWidth := size.Width - width
		diffHeight := size.Height - height
		currDiffWidth := targetWidth - width
		currDiffHeight := targetHeight - height

		diff := diffWidth + diffHeight
		currDiff := currDiffWidth + currDiffHeight

		if !foundFirst || diff < currDiff || lastSize {
			foundFirst = true
			targetWidth = size.Width
			targetHeight = size.Height
		}
	}

	log = log.WithFields(logrus.Fields{
		"targetWidth": targetWidth,
		"targetHeight": targetHeight,
	})
	log.Info("Looking up thumbnail")

	thumb, err := db.GetThumbnail(ctx, media.Origin, media.MediaId, targetWidth, targetHeight, method)
	if err != nil && err != sql.ErrNoRows {
		log.Error("Unexpected error processing thumbnail lookup: " + err.Error())
		return thumb, err
	}
	if err != sql.ErrNoRows {
		log.Info("Found existing thumbnail")
		return thumb, nil
	}

	if media.SizeBytes > c.Thumbnails.MaxSourceBytes {
		log.Warn("Media too large to thumbnail")
		return thumb, errors.New("cannot thumbnail, image too large")
	}

	log.Info("Generating new thumbnail")
	return generateThumbnail(ctx, media, targetWidth, targetHeight, method, c, db, log)
}

func generateThumbnail(ctx context.Context, media types.Media, width int, height int, method string, c config.MediaRepoConfig, db storage.Database, log *logrus.Entry) (types.Thumbnail, error) {
	thumb := &types.Thumbnail{
		Origin:     media.Origin,
		MediaId:    media.MediaId,
		Width:      width,
		Height:     height,
		Method:     method,
		CreationTs: time.Now().UnixNano() / 1000000,
		// ContentType:
		// Location:
		// SizeBytes:
	}

	src, err := imaging.Open(media.Location)
	if err != nil {
		return *thumb, err
	}

	srcWidth := src.Bounds().Max.X
	srcHeight := src.Bounds().Max.Y

	aspectRatio := float32(srcHeight) / float32(srcWidth)
	targetAspectRatio := float32(width) / float32(height)
	if aspectRatio == targetAspectRatio {
		// Highly unlikely, but if the aspect ratios match then just resize
		method = "scale"
		log.Info("Aspect ratio is the same, converting method to 'scale'")
	}

	if srcWidth <= width && srcHeight <= height {
		// Image is too small - don't upscale
		thumb.ContentType = media.ContentType
		thumb.Location = media.Location
		thumb.SizeBytes = media.SizeBytes
		log.Warn("Image too small, returning raw image")
		return *thumb, nil
	}

	if method == "scale" {
		src = imaging.Fit(src, width, height, imaging.Lanczos)
	} else if method == "crop" {
		src = imaging.Fill(src, width, height, imaging.Center, imaging.Lanczos)
	} else {
		log.Error("Unrecognized thumbnail method: " + method)
		return *thumb, errors.New("unrecognized method: " + method)
	}

	// Put the image bytes into a memory buffer
	imgData := &bytes.Buffer{}
	err = imaging.Encode(imgData, src, imaging.PNG)
	if err != nil {
		log.Error("Unexpected error encoding thumbnail: " + err.Error())
		return *thumb, err
	}

	// Reset the buffer pointer and store the file
	location, err := storage.PersistFile(ctx, imgData, c, db)
	if err != nil {
		log.Error("Unexpected error saving thumbnail: " + err.Error())
		return *thumb, err
	}

	fileSize, err := util.FileSize(location)
	if err != nil {
		log.Error("Unexpected error getting the size of the thumbnail: " + err.Error())
		return *thumb, err
	}

	thumb.Location = location
	thumb.ContentType = "image/png"
	thumb.SizeBytes = fileSize

	err = db.InsertThumbnail(ctx, thumb)
	if err != nil {
		log.Error("Unexpected error caching thumbnail: " + err.Error())
		return *thumb, err
	}

	return *thumb, nil
}