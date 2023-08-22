package thumbnails

import (
	"errors"
	"io"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/datastores"
	"github.com/turt2live/matrix-media-repo/metrics"
	"github.com/turt2live/matrix-media-repo/pipelines/_steps/datastore_op"
	"github.com/turt2live/matrix-media-repo/pipelines/_steps/download"
	"github.com/turt2live/matrix-media-repo/pool"
	"github.com/turt2live/matrix-media-repo/thumbnailing"
	"github.com/turt2live/matrix-media-repo/thumbnailing/m"
	"github.com/turt2live/matrix-media-repo/util"
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

		mediaStream, err := download.OpenStream(ctx, mediaRecord.Locatable, -1, -1)
		if err != nil {
			ch <- generateResult{err: err}
			return
		}
		fixedContentType := util.FixContentType(mediaRecord.ContentType)

		i, err := thumbnailing.GenerateThumbnail(mediaStream, fixedContentType, width, height, method, animated, ctx)
		if err != nil {
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
	err = database.GetInstance().Thumbnails.Prepare(ctx).Insert(newRecord)
	if err != nil {
		defer thumbStream.Close()
		return nil, nil, err
	}

	return newRecord, thumbStream, nil
}
