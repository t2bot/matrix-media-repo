package handlers

import (
	"bytes"
	"errors"

	"github.com/disintegration/imaging"
	"github.com/turt2live/matrix-media-repo/rcontext"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/stores"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type GeneratedThumbnail struct {
	ContentType  string
	DiskLocation string
	SizeBytes    int64
}

type Thumbnailer struct {
	ThumbnailStore stores.ThumbnailStore
	Info           rcontext.RequestInfo
}

func (t *Thumbnailer) GenerateThumbnail(media types.Media, width int, height int, method string) (GeneratedThumbnail, error) {
	thumb := &GeneratedThumbnail{}

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
		t.Info.Log.Info("Aspect ratio is the same, converting method to 'scale'")
	}

	if srcWidth <= width && srcHeight <= height {
		// Image is too small - don't upscale
		thumb.ContentType = media.ContentType
		thumb.DiskLocation = media.Location
		thumb.SizeBytes = media.SizeBytes
		t.Info.Log.Warn("Image too small, returning raw image")
		return *thumb, nil
	}

	if method == "scale" {
		src = imaging.Fit(src, width, height, imaging.Lanczos)
	} else if method == "crop" {
		src = imaging.Fill(src, width, height, imaging.Center, imaging.Lanczos)
	} else {
		t.Info.Log.Error("Unrecognized thumbnail method: " + method)
		return *thumb, errors.New("unrecognized method: " + method)
	}

	// Put the image bytes into a memory buffer
	imgData := &bytes.Buffer{}
	err = imaging.Encode(imgData, src, imaging.PNG)
	if err != nil {
		t.Info.Log.Error("Unexpected error encoding thumbnail: " + err.Error())
		return *thumb, err
	}

	// Reset the buffer pointer and store the file
	location, err := storage.PersistFile(imgData, t.Info.Config, t.Info.Context, &t.Info.Db)
	if err != nil {
		t.Info.Log.Error("Unexpected error saving thumbnail: " + err.Error())
		return *thumb, err
	}

	fileSize, err := util.FileSize(location)
	if err != nil {
		t.Info.Log.Error("Unexpected error getting the size of the thumbnail: " + err.Error())
		return *thumb, err
	}

	thumb.DiskLocation = location
	thumb.ContentType = "image/png"
	thumb.SizeBytes = fileSize

	return *thumb, nil
}
