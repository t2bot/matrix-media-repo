package download

import (
	"errors"
	"io"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/datastores"
	"github.com/turt2live/matrix-media-repo/redislib"
)

type limitedCloser struct {
	io.ReadCloser
	lm io.Reader
	rs io.ReadCloser
}

func (r limitedCloser) Read(p []byte) (int, error) {
	return r.lm.Read(p)
}

func (r limitedCloser) Close() error {
	return r.rs.Close()
}

func OpenStream(ctx rcontext.RequestContext, media *database.Locatable, startByte int64, endByte int64) (io.ReadCloser, error) {
	reader, err := redislib.TryGetMedia(ctx, media.Sha256Hash, startByte, endByte)
	if err != nil || reader != nil {
		ctx.Log.Debugf("Got %s from cache", media.Sha256Hash)
		return io.NopCloser(reader), err
	}

	ds, ok := datastores.Get(ctx, media.DatastoreId)
	if !ok {
		return nil, errors.New("unable to locate datastore for media")
	}

	rsc, err := datastores.Download(ctx, ds, media.Location)
	if err != nil {
		return nil, err
	}

	return CreateLimitedStream(ctx, rsc, startByte, endByte)
}

func CreateLimitedStream(ctx rcontext.RequestContext, r io.ReadCloser, startByte int64, endByte int64) (io.ReadCloser, error) {
	if startByte >= 0 {
		if rsc, ok := r.(io.ReadSeekCloser); ok {
			if _, err := rsc.Seek(startByte, io.SeekStart); err != nil {
				err2 := rsc.Close()
				if err2 != nil {
					ctx.Log.Errorf("Error while closing datastore stream due to other error: %s", err2)
					sentry.CaptureException(err2)
				}
				return nil, err
			}
		} else {
			_, err := io.CopyN(io.Discard, r, startByte)
			if err != nil {
				err2 := r.Close()
				if err2 != nil {
					ctx.Log.Errorf("Error while closing datastore stream due to other error: %s", err2)
					sentry.CaptureException(err2)
				}
				return nil, err
			}
		}
	}

	var lm io.Reader = r
	if endByte >= 1 {
		if startByte < 0 {
			startByte = 0
		}
		lm = io.LimitReader(r, endByte-startByte)
	}
	return &limitedCloser{lm: lm, rs: r}, nil
}
