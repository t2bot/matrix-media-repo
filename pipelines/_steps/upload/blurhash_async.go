package upload

import (
	"image"
	"io"

	"github.com/buckket/go-blurhash"
	"github.com/disintegration/imaging"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
)

func CalculateBlurhashAsync(ctx rcontext.RequestContext, reader io.Reader, sha256hash string) chan struct{} {
	var err error
	opChan := make(chan struct{})
	go func() {
		//goland:noinspection GoUnhandledErrorResult
		defer io.Copy(io.Discard, reader) // we need to flush the reader as we might end up blocking the upload
		defer close(opChan)

		if !ctx.Config.Features.MSC2448Blurhash.Enabled {
			return
		}

		var img image.Image
		img, err = imaging.Decode(reader)
		if err != nil {
			ctx.Log.Debug("Skipping blurhash on this upload due to error: ", err)
			return
		}

		// Resize
		img = imaging.Fill(img, ctx.Config.Features.MSC2448Blurhash.GenerateWidth, ctx.Config.Features.MSC2448Blurhash.GenerateHeight, imaging.Center, imaging.Lanczos)

		// Calculate the blurhash
		var bh string
		bh, err = blurhash.Encode(ctx.Config.Features.MSC2448Blurhash.XComponents, ctx.Config.Features.MSC2448Blurhash.YComponents, img)
		if err != nil {
			ctx.Log.Debug("Skipping blurhash on this upload due to error: ", err)
			return
		}

		// Insert
		err = database.GetInstance().Blurhashes.Prepare(ctx).Insert(sha256hash, bh)
		if err != nil {
			ctx.Log.Debug("Skipping blurhash on this upload due to error: ", err)
			return
		}
	}()
	return opChan
}
