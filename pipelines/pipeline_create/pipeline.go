package pipeline_create

import (
	"time"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/pipelines/_steps/quota"
	"github.com/t2bot/matrix-media-repo/pipelines/_steps/upload"
)

const DefaultExpirationTime = 0

func Execute(ctx rcontext.RequestContext, origin string, userId string, expirationTime int64) (*database.DbExpiringMedia, error) {
	// Step 1: Check quota
	if err := quota.Check(ctx, userId, quota.MaxPending); err != nil {
		return nil, err
	}

	// Step 2: Generate media ID
	mediaId, err := upload.GenerateMediaId(ctx, origin)
	if err != nil {
		return nil, err
	}

	// Step 3: Insert record of expiration
	if expirationTime == DefaultExpirationTime {
		expirationTime = ctx.Config.Uploads.MaxAgeSeconds
	}
	expires := time.Now().Add(time.Duration(expirationTime) * time.Second)
	expiresTS := expires.UnixMilli()
	if err = database.GetInstance().ExpiringMedia.Prepare(ctx).Insert(origin, mediaId, userId, expiresTS); err != nil {
		return nil, err
	}

	// Step 4: Return database record
	return &database.DbExpiringMedia{
		Origin:    origin,
		MediaId:   mediaId,
		UserId:    userId,
		ExpiresTs: expiresTS,
	}, nil
}
