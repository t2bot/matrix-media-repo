package url_preview

import (
	"io"

	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/datastores"
	"github.com/t2bot/matrix-media-repo/pipelines/pipeline_upload"
	"github.com/t2bot/matrix-media-repo/thumbnailing"
	"github.com/t2bot/matrix-media-repo/url_previewing/m"
	"github.com/t2bot/matrix-media-repo/util"
)

func UploadImage(ctx rcontext.RequestContext, image *m.PreviewImage, onHost string, userId string, forRecord *database.DbUrlPreview) {
	if image == nil || image.Data == nil {
		return
	}

	defer image.Data.Close()
	pr, pw := io.Pipe()
	tee := io.TeeReader(image.Data, pw)
	mediaChan := make(chan *database.DbMedia)
	defer close(mediaChan)
	go func() {
		media, err := pipeline_upload.Execute(ctx, onHost, "", io.NopCloser(tee), image.ContentType, image.Filename, userId, datastores.LocalMediaKind)
		if err != nil {
			_ = pw.CloseWithError(err)
		} else {
			_ = pw.Close()
		}
		go func() {
			defer func() {
				recover() // consume write-to-closed-channel errors
			}()
			mediaChan <- media
		}()
	}()

	w := 0
	h := 0
	g, r, err := thumbnailing.GetGenerator(pr, image.ContentType, false)
	_, _ = io.Copy(io.Discard, pr)
	if err != nil {
		ctx.Log.Warn("Non-fatal error handling URL preview thumbnail: ", err)
		sentry.CaptureException(err)
		return
	}
	if g != nil {
		_, w, h, err = g.GetOriginDimensions(r, image.ContentType, ctx)
		if err != nil {
			ctx.Log.Warn("Non-fatal error getting URL preview thumbnail dimensions: ", err)
			sentry.CaptureException(err)
		}
	}

	record := <-mediaChan
	if record == nil {
		return
	}

	forRecord.ImageMxc = util.MxcUri(record.Origin, record.MediaId)
	forRecord.ImageType = record.ContentType
	forRecord.ImageSize = record.SizeBytes
	forRecord.ImageWidth = w
	forRecord.ImageHeight = h
}
