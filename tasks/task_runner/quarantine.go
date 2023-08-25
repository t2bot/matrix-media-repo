package task_runner

import (
	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/redislib"
	"github.com/turt2live/matrix-media-repo/util"
)

type QuarantineRecord struct {
	Origin  string
	MediaId string
}

type QuarantineThis struct {
	MxcUris []string
	Single  *QuarantineRecord
	DbMedia []*database.DbMedia
}

// QuarantineMedia returns (count quarantined, error)
func QuarantineMedia(ctx rcontext.RequestContext, onlyHost string, toHandle *QuarantineThis) (int64, error) {
	records, err := resolveMedia(ctx, onlyHost, toHandle) // records are roughly safe to rely on host-wise
	if err != nil {
		return 0, err
	}

	metadataDb := database.GetInstance().MetadataView.Prepare(ctx)
	total := int64(0)
	for _, r := range records {
		if onlyHost != "" && onlyHost != r.Origin {
			continue
		}

		count := int64(0)
		if onlyHost != "" {
			count, err = metadataDb.UpdateQuarantineByHashAndOrigin(r.Origin, r.Sha256Hash, true)
		} else {
			count, err = metadataDb.UpdateQuarantineByHash(r.Sha256Hash, true)
		}
		total += count
		if err != nil {
			return total, err
		}

		err = redislib.DeleteMedia(ctx, r.Sha256Hash)
		if err != nil {
			ctx.Log.Warn("Error while deleting cached media: ", err)
			sentry.CaptureException(err)
		}
	}

	return total, nil
}

func resolveMedia(ctx rcontext.RequestContext, onlyHost string, toHandle *QuarantineThis) ([]*database.DbMedia, error) {
	db := database.GetInstance().Media.Prepare(ctx)

	records := make([]*database.DbMedia, 0)
	if toHandle.DbMedia != nil {
		records = append(records, toHandle.DbMedia...)
	}
	if toHandle.Single != nil && (onlyHost == "" || toHandle.Single.Origin == onlyHost) {
		r, err := db.GetById(toHandle.Single.Origin, toHandle.Single.MediaId)
		if err != nil {
			return nil, err
		}
		if r != nil {
			records = append(records, r)
		}
	}
	if toHandle.MxcUris != nil {
		for _, mxc := range toHandle.MxcUris {
			origin, mediaId, err := util.SplitMxc(mxc)
			if onlyHost != "" && origin != onlyHost {
				continue
			}
			if err != nil {
				return nil, err
			}
			r, err := db.GetById(origin, mediaId)
			if err != nil {
				return nil, err
			}
			if r != nil {
				records = append(records, r)
			}
		}
	}

	return records, nil
}
