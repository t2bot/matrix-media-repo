package thumbnails

import (
	"errors"
	"io"
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/datastores"
	"github.com/t2bot/matrix-media-repo/metrics"
	"github.com/t2bot/matrix-media-repo/pipelines/_steps/datastore_op"
	"github.com/t2bot/matrix-media-repo/pipelines/_steps/download"
	"github.com/t2bot/matrix-media-repo/pool"
	"github.com/t2bot/matrix-media-repo/thumbnailing"
	"github.com/t2bot/matrix-media-repo/thumbnailing/m"
	"github.com/t2bot/matrix-media-repo/util"
)

type generateResult struct {
	i   *m.Thumbnail
	err error
}

func Generate(ctx rcontext.RequestContext, mediaRecord *database.DbMedia, width int, height int, method string, animated bool) (*database.DbThumbnail, io.ReadCloser, error) {
	ch := make(chan generateResult)
	defer close(ch)
	fn := func() {
		metric := metrics.ThumbnailsGenerated.With(prometheus.Labels{
			"width":    strconv.Itoa(width),
			"height":   strconv.Itoa(height),
			"method":   method,
			"animated": strconv.FormatBool(animated),
			"origin":   mediaRecord.Origin,
		})

		mediaStream, err := download.OpenStream(ctx, mediaRecord.Locatable)
		if err != nil {
			ch <- generateResult{err: err}
			return
		}
		fixedContentType := util.FixContentType(mediaRecord.ContentType)

		i, err := thumbnailing.GenerateThumbnail(mediaStream, fixedContentType, width, height, method, animated, ctx)
		if err != nil {
			if i != nil && i.Reader != nil {
				err2 := i.Reader.Close()
				if err2 != nil {
					ctx.Log.Warn("Non-fatal error cleaning up thumbnail stream: ", err2)
				}
			}
			if errors.Is(err, common.ErrMediaDimensionsTooSmall) {
				metric.Inc()
			}
			ch <- generateResult{err: err}
			return
		}

		metric.Inc()
		ch <- generateResult{i: i}
	}

	if err := pool.ThumbnailQueue.Schedule(fn); err != nil {
		return nil, nil, err
	}
	res := <-ch
	if res.err != nil {
		return nil, nil, res.err
	}
	if res.i == nil {
		// Couldn't generate a thumbnail
		return nil, nil, common.ErrMediaNotFound
	}

	// At this point, res.i is our thumbnail

	// Quickly check to see if we already have a database record for this thumbnail. We do this because predicting
	// what the thumbnailer will generate is non-trivial, but it might generate a conflicting thumbnail (particularly
	// when `defaultAnimated` is `true`.
	db := database.GetInstance().Thumbnails.Prepare(ctx)
	if res.i.Animated != animated { // this is the only thing that could have changed during generation
		existingRecord, err := db.GetByParams(mediaRecord.Origin, mediaRecord.MediaId, width, height, method, res.i.Animated)
		if err != nil {
			return nil, nil, err
		}
		if existingRecord != nil {
			ctx.Log.Debug("Found existing record for parameters - discarding generated thumbnail")
			defer res.i.Reader.Close()

			// Optimization: prevent future generator waste by inserting an `animated=true` record for static media,
			// since we won't ever generate an animated version. This is safe because to get here the thumbnail needed
			// to be requested as animated, but the generated one wasn't. This implies we are trying to animate a static
			// image, which doesn't work.
			if !res.i.Animated {
				existingRecord.Animated = true
				// we don't modify the creation time, so it expires at a sane point in history
				err = db.Insert(existingRecord)
				if err != nil {
					ctx.Log.Warn("Non-fatal error while optimizing future thumbnail lookups: ", err)
					sentry.CaptureException(err)
				}
			}

			rsc, err := download.OpenStream(ctx, existingRecord.Locatable)
			if err != nil {
				return nil, nil, err
			}
			return existingRecord, rsc, nil
		}
	}

	// We don't have an existing record. Store the stream and insert a record.
	thumbMediaRecord, thumbStream, err := datastore_op.PutAndReturnStream(ctx, ctx.Request.Host, "", res.i.Reader, res.i.ContentType, "", datastores.ThumbnailsKind)
	if err != nil {
		return nil, nil, err
	}

	// Create a DbThumbnail
	newRecord := &database.DbThumbnail{
		Origin:      mediaRecord.Origin,
		MediaId:     mediaRecord.MediaId,
		ContentType: thumbMediaRecord.ContentType,
		Width:       width,
		Height:      height,
		Method:      method,
		Animated:    res.i.Animated,
		SizeBytes:   thumbMediaRecord.SizeBytes,
		CreationTs:  thumbMediaRecord.CreationTs,
		Locatable: &database.Locatable{
			Sha256Hash:  thumbMediaRecord.Sha256Hash,
			DatastoreId: thumbMediaRecord.DatastoreId,
			Location:    thumbMediaRecord.Location,
		},
	}
	err = db.Insert(newRecord)
	if err != nil {
		defer thumbStream.Close()
		return nil, nil, err
	}

	return newRecord, thumbStream, nil
}
