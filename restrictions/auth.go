package restrictions

import (
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
)

func DoesMediaRequireAuth(ctx rcontext.RequestContext, origin string, mediaId string) (bool, error) {
	restrictions, err := database.GetInstance().RestrictedMedia.Prepare(ctx).GetAllForId(origin, mediaId)
	if err != nil {
		return false, err
	}
	for _, restriction := range restrictions {
		if restriction.Condition == database.RestrictedRequiresAuth {
			return restriction.ConditionValue == "true", nil
		}
	}
	return false, nil
}

func SetMediaRequiresAuth(ctx rcontext.RequestContext, origin string, mediaId string) error {
	return database.GetInstance().RestrictedMedia.Prepare(ctx).Insert(origin, mediaId, database.RestrictedRequiresAuth, "true")
}
