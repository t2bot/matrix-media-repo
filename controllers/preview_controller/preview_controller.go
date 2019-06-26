package preview_controller

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/globals"
	"github.com/turt2live/matrix-media-repo/controllers/preview_controller/acl"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/stores"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

func GetPreview(urlStr string, onHost string, forUserId string, atTs int64, ctx context.Context, log *logrus.Entry) (*types.UrlPreview, error) {
	atTs = stores.GetBucketTs(atTs)
	cacheKey := fmt.Sprintf("%d_%s/%s", atTs, onHost, urlStr)
	v, _, err := globals.DefaultRequestGroup.DoWithoutPost(cacheKey, func() (interface{}, error) {

		log = log.WithFields(logrus.Fields{
			"preview_controller_at_ts": atTs,
		})

		db := storage.GetDatabase().GetUrlStore(ctx, log)

		cached, err := db.GetPreview(urlStr, atTs)
		if err != nil && err != sql.ErrNoRows {
			log.Error("Error getting cached URL preview: ", err.Error())
			return nil, err
		}
		if err != sql.ErrNoRows {
			log.Info("Returning cached URL preview")
			return cachedPreviewToReal(cached)
		}

		now := util.NowMillis()
		if (now - atTs) > 60000 {
			// Because we don't have a cached preview, we'll use the current time as the preview time.
			// We also give a 60 second buffer so we don't cause an infinite loop (considering we're
			// calling ourselves), and to give a lenient opportunity for slow execution.
			return GetPreview(urlStr, onHost, forUserId, now, ctx, log)
		}

		log.Info("Preview not cached - fetching resource")

		urlToPreview, err := acl.ValidateUrlForPreview(urlStr, ctx, log)
		if err != nil {
			return nil, err
		}

		previewChan := getResourceHandler().GeneratePreview(urlToPreview, forUserId, onHost)
		defer close(previewChan)

		result := <-previewChan
		return result.preview, result.err
	})

	return v.(*types.UrlPreview), err
}

func cachedPreviewToReal(cached *types.CachedUrlPreview) (*types.UrlPreview, error) {
	if cached.ErrorCode == common.ErrCodeInvalidHost {
		return nil, common.ErrInvalidHost
	} else if cached.ErrorCode == common.ErrCodeHostNotFound {
		return nil, common.ErrHostNotFound
	} else if cached.ErrorCode == common.ErrCodeHostBlacklisted {
		return nil, common.ErrHostBlacklisted
	} else if cached.ErrorCode == common.ErrCodeNotFound {
		return nil, common.ErrMediaNotFound
	} else if cached.ErrorCode == common.ErrCodeUnknown {
		return nil, errors.New("unknown error")
	}

	return cached.Preview, nil
}
