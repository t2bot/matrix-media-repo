package quota

import (
	"errors"

	"github.com/ryanuber/go-glob"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
)

type Type int64

const (
	MaxBytes   Type = 0
	MaxPending Type = 1
	MaxCount   Type = 2
)

func Check(ctx rcontext.RequestContext, userId string, quotaType Type) error {
	limit, err := Limit(ctx, userId, quotaType)
	if err != nil {
		return err
	}

	if quotaType == MaxBytes || quotaType == MaxCount || quotaType == MaxPending {
		if limit <= 0 {
			return nil
		}
	}

	count, err := Current(ctx, userId, quotaType)
	if err != nil {
		return err
	}
	if count < limit {
		return nil
	} else {
		ctx.Log.Debugf("Quota %d current=%d limit=%d", int64(quotaType), count, limit)
		return common.ErrQuotaExceeded
	}
}

func Current(ctx rcontext.RequestContext, userId string, quotaType Type) (int64, error) {
	var count int64
	var err error
	if quotaType == MaxBytes {
		count, err = database.GetInstance().UserStats.Prepare(ctx).UserUploadedBytes(userId)
	} else if quotaType == MaxPending {
		count, err = database.GetInstance().ExpiringMedia.Prepare(ctx).ByUserCount(userId)
	} else if quotaType == MaxCount {
		count, err = database.GetInstance().Media.Prepare(ctx).ByUserCount(userId)
	} else {
		return 0, errors.New("missing current count for quota type - contact developer")
	}

	return count, err
}

func CanUpload(ctx rcontext.RequestContext, userId string, bytes int64) error {
	// We can't use Check() for MaxBytes because we're testing limit+to_be_uploaded_size
	limit, err := Limit(ctx, userId, MaxBytes)
	if err != nil {
		return err
	}
	if limit < 0 {
		return nil
	}

	count, err := Current(ctx, userId, MaxBytes)
	if err != nil {
		return err
	}

	if (count + bytes) > limit {
		ctx.Log.Debugf("Quota %s current=%d bytes=%d limit=%d", "CanUpload", count, bytes, limit)
		return common.ErrQuotaExceeded
	}

	if err = Check(ctx, userId, MaxCount); err != nil {
		return err
	}

	return nil
}

func Limit(ctx rcontext.RequestContext, userId string, quotaType Type) (int64, error) {
	if !ctx.Config.Uploads.Quota.Enabled {
		return defaultLimit(ctx, quotaType)
	}

	db := database.GetInstance().UserStats.Prepare(ctx)
	record, err := db.GetUserQuota([]string{userId})
	if err != nil {
		ctx.Log.Warn("Error querying DB quota for user " + userId + ": " + err.Error())
	} else if len(record) == 0 {
		ctx.Log.Warn("User " + userId + " does not exist in DB. Skipping DB quota check...")
	} else {
		// DB quotas takes precedence over config quotas if value is not -1
		quota := record[0].UserQuota
		switch quotaType {
		case MaxBytes:
			if quota.MaxBytes > 0 {
				return quota.MaxBytes, nil
			} else if quota.MaxBytes == 0 {
				return defaultLimit(ctx, quotaType)
			}
		case MaxPending:
			if quota.MaxPending > 0 {
				return quota.MaxPending, nil
			} else if quota.MaxPending == 0 {
				return defaultLimit(ctx, quotaType)
			}
		case MaxCount:
			if quota.MaxFiles > 0 {
				return quota.MaxFiles, nil
			} else if quota.MaxFiles == 0 {
				return defaultLimit(ctx, quotaType)
			}
		default:
			return 0, errors.New("missing db switch for quota type - contact developer")
		}
	}

	for _, q := range ctx.Config.Uploads.Quota.UserQuotas {
		if glob.Glob(q.Glob, userId) {
			if quotaType == MaxBytes {
				return q.MaxBytes, nil
			} else if quotaType == MaxPending {
				return q.MaxPending, nil
			} else if quotaType == MaxCount {
				return q.MaxFiles, nil
			} else {
				return 0, errors.New("missing glob switch for quota type - contact developer")
			}
		}
	}

	return defaultLimit(ctx, quotaType)
}

func defaultLimit(ctx rcontext.RequestContext, quotaType Type) (int64, error) {
	if quotaType == MaxBytes {
		return -1, nil
	} else if quotaType == MaxPending {
		return ctx.Config.Uploads.MaxPending, nil
	} else if quotaType == MaxCount {
		return 0, nil
	}
	return 0, errors.New("no default for quota type - contact developer")
}
