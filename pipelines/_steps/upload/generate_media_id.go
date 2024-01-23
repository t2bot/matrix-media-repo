package upload

import (
	"errors"

	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/util/ids"
)

func GenerateMediaId(ctx rcontext.RequestContext, origin string) (string, error) {
	if config.Runtime.IsImportProcess {
		return "", errors.New("media IDs should not be generated from import processes")
	}
	heldDb := database.GetInstance().HeldMedia.Prepare(ctx)
	mediaDb := database.GetInstance().Media.Prepare(ctx)
	reservedDb := database.GetInstance().ReservedMedia.Prepare(ctx)
	var mediaId string
	var err error
	var exists bool
	attempts := 0
	for {
		attempts += 1
		if attempts > 10 {
			return "", errors.New("internal limit reached: unable to generate media ID")
		}

		mediaId, err = ids.NewUniqueId()

		err = heldDb.TryInsert(origin, mediaId, database.ForCreateHeldReason)
		if err != nil {
			return "", err
		}

		// Check if there's a media table record for this media as well (there shouldn't be)
		exists, err = mediaDb.IdExists(origin, mediaId)
		if err != nil {
			return "", err
		}
		if exists {
			continue
		}

		// Also check to see if the media ID is reserved due to a past action
		exists, err = reservedDb.IdExists(origin, mediaId)
		if err != nil {
			return "", err
		}
		if exists {
			continue
		}

		return mediaId, nil
	}
	return "", errors.New("internal limit reached: fell out of media ID generation loop")
}
