package quota

import (
	"database/sql"

	"github.com/ryanuber/go-glob"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/storage"
)

func IsUserWithinQuota(ctx rcontext.RequestContext, userId string) (bool, error) {
	if !ctx.Config.Uploads.Quota.Enabled {
		return true, nil
	}

	db := storage.GetDatabase().GetMetadataStore(ctx)
	stat, err := db.GetUserStats(userId)
	if err == sql.ErrNoRows {
		return true, nil // no stats == within quota
	}
	if err != nil {
		return false, err
	}

	for _, q := range ctx.Config.Uploads.Quota.UserQuotas {
		if glob.Glob(q.Glob, userId) {
			if q.MaxBytes == 0 {
				return true, nil // infinite quota
			}
			return stat.UploadedBytes < q.MaxBytes, nil
		}
	}

	return true, nil // no rules == no quota
}

func GetUserUploadMaxSizeBytes(ctx rcontext.RequestContext, userId string) int64 {
	for _, u := range ctx.Config.Uploads.UsersMaxSizeBytes {
		if glob.Glob(u.Glob, userId) {
			if u.MaxBytes == 0 {
				return ctx.Config.Uploads.MaxSizeBytes
			} else if u.MaxBytes < 0 {
				return 0
			} else {
				return u.MaxBytes
			}
		}
	}

}
