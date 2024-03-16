package upload

import (
	"io"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/redislib"
)

func PopulateCacheAsync(ctx rcontext.RequestContext, reader io.Reader, size int64, sha256hash string) chan struct{} {
	var err error
	opChan := make(chan struct{})
	go func() {
		//goland:noinspection GoUnhandledErrorResult
		defer io.Copy(io.Discard, reader) // we need to flush the reader as we might end up blocking the upload
		defer close(opChan)

		err = redislib.StoreMedia(ctx, sha256hash, reader, size)
		if err != nil {
			ctx.Log.Debug("Not populating cache due to error: ", err)
			sentry.CaptureException(err)
			return
		}
	}()
	return opChan
}
