package upload

import (
	"errors"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/util/ids"
)

func GenerateMediaId(ctx rcontext.RequestContext, origin string) (string, error) {
	db := database.GetInstance().ReservedMedia.Prepare(ctx)
	var mediaId string
	var err error
	attempts := 0
	for true {
		attempts += 1
		if attempts > 10 {
			return "", errors.New("internal limit reached: unable to generate media ID")
		}

		mediaId, err = ids.NewUniqueId()

		err = db.TryInsert(origin, mediaId, database.ForCreateReserveReason)
		if err != nil {
			return "", err
		}

		// Check if there's a media table record for this media as well (there shouldn't be)
		return mediaId, nil // TODO: @@TR - This
	}
	return "", errors.New("internal limit reached: fell out of media ID generation loop")
}
