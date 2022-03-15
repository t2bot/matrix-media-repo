package info_controller

import (
	"bytes"
	"image/png"

	"github.com/buckket/go-blurhash"
	"github.com/disintegration/imaging"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/download_controller"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util/cleanup"
)

func GetOrCalculateBlurhash(media *types.Media, rctx rcontext.RequestContext) (string, error) {
	rctx.Log.Info("Attempting fetch of blurhash for sha256 of " + media.Sha256Hash)
	db := storage.GetDatabase().GetMetadataStore(rctx)
	cached, err := db.GetBlurhash(media.Sha256Hash)
	if err != nil {
		return "", err
	}

	if cached != "" {
		rctx.Log.Info("Returning cached blurhash: " + cached)
		return cached, nil
	}

	rctx.Log.Info("Getting minimal media record to calculate blurhash")
	minMedia, err := download_controller.FindMinimalMediaRecord(media.Origin, media.MediaId, true, nil, rctx)
	if err != nil {
		return "", err
	}
	defer cleanup.DumpAndCloseStream(minMedia.Stream)

	// No cached blurhash: calculate one
	rctx.Log.Info("Decoding image for blurhash calculation")
	imgSrc, err := imaging.Decode(minMedia.Stream)
	if err != nil {
		return "", err
	}

	// Resize the image to make the blurhash a bit more reasonable to calculate
	rctx.Log.Info("Resizing image for blurhash (faster calculation)")
	smallImg := imaging.Fill(imgSrc, rctx.Config.Features.MSC2448Blurhash.GenerateWidth, rctx.Config.Features.MSC2448Blurhash.GenerateHeight, imaging.Center, imaging.Lanczos)
	imgBuf := &bytes.Buffer{}
	err = imaging.Encode(imgBuf, smallImg, imaging.PNG)
	if err != nil {
		return "", err
	}
	decoded, err := png.Decode(imgBuf)
	if err != nil {
		return "", err
	}

	rctx.Log.Info("Calculating blurhash")
	encoded, err := blurhash.Encode(rctx.Config.Features.MSC2448Blurhash.XComponents, rctx.Config.Features.MSC2448Blurhash.YComponents, decoded)
	if err != nil {
		return "", err
	}

	// Save the blurhash for next time
	rctx.Log.Infof("Saving blurhash %s and returning", encoded)
	err = db.InsertBlurhash(media.Sha256Hash, encoded)
	if err != nil {
		return "", err
	}

	return encoded, nil
}
