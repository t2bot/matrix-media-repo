package upload

import (
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
)

func FindRecord(ctx rcontext.RequestContext, hash string, userId string, contentType string, fileName string) (*database.DbMedia, bool, error) {
	mediaDb := database.GetInstance().Media.Prepare(ctx)
	records, err := mediaDb.GetByHash(hash)
	if err != nil {
		return nil, false, err
	}
	var perfectMatch *database.DbMedia = nil
	var hashMatch *database.DbMedia = nil
	for _, r := range records {
		if hashMatch == nil {
			hashMatch = r
		}
		if r.UserId == userId && r.ContentType == r.ContentType && r.UploadName == fileName {
			perfectMatch = r
			break
		}
	}
	if perfectMatch != nil {
		return perfectMatch, true, nil
	} else {
		return hashMatch, false, nil
	}
}
