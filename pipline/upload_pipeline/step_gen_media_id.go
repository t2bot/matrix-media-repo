package upload_pipeline

import (
	"errors"
	"strconv"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/util"
)

var recentMediaIds = cache.New(30*time.Second, 60*time.Second)

func generateMediaID(ctx rcontext.RequestContext, origin string) (string, error) {
	metadataDb := storage.GetDatabase().GetMetadataStore(ctx)
	mediaTaken := true
	var mediaId string
	var err error
	attempts := 0
	for mediaTaken {
		attempts += 1
		if attempts > 10 {
			return "", errors.New("failed to generate a media ID after 10 rounds")
		}

		mediaId, err = util.GenerateRandomString(64)
		if err != nil {
			return "", err
		}
		mediaId, err = util.GetSha1OfString(mediaId + strconv.FormatInt(util.NowMillis(), 10))
		if err != nil {
			return "", err
		}

		// Because we use the current time in the media ID, we don't need to worry about
		// collisions from the database.
		if _, present := recentMediaIds.Get(mediaId); present {
			mediaTaken = true
			continue
		}

		mediaTaken, err = metadataDb.IsReserved(origin, mediaId)
		if err != nil {
			return "", err
		}
	}

	_ = recentMediaIds.Add(mediaId, true, cache.DefaultExpiration)

	return mediaId, nil
}
