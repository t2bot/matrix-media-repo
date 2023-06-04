package quota

import (
	"errors"

	"github.com/ryanuber/go-glob"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
)

type Type int64

const (
	MaxBytes   Type = 0
	MaxPending Type = 1
)

func Check(ctx rcontext.RequestContext, userId string, quotaType Type) error {
	limit, err := Limit(ctx, userId, quotaType)
	if err != nil {
		return err
	}

	var count int64
	if quotaType == MaxBytes {
		if limit < 0 {
			return nil
		}
		count, err = database.GetInstance().UserStats.Prepare(ctx).UserUploadedBytes(userId)
	} else if quotaType == MaxPending {
		count, err = database.GetInstance().ExpiringMedia.Prepare(ctx).ByUserCount(userId)
	} else {
		return errors.New("missing check for quota type - contact developer")
	}

	if err != nil {
		return err
	}
	if count < limit {
		return nil
	} else {
		return common.ErrQuotaExceeded
	}
}

func CanUpload(ctx rcontext.RequestContext, userId string, bytes int64) error {
	limit, err := Limit(ctx, userId, MaxBytes)
	if err != nil {
		return err
	}
	if limit < 0 {
		return nil
	}

	count, err := database.GetInstance().UserStats.Prepare(ctx).UserUploadedBytes(userId)
	if err != nil {
		return err
	}

	if (count + bytes) > limit {
		return common.ErrQuotaExceeded
	}

	return nil
}

func Limit(ctx rcontext.RequestContext, userId string, quotaType Type) (int64, error) {
	if !ctx.Config.Uploads.Quota.Enabled {
		return defaultLimit(ctx, quotaType)
	}

	for _, q := range ctx.Config.Uploads.Quota.UserQuotas {
		if glob.Glob(q.Glob, userId) {
			if quotaType == MaxBytes {
				return q.MaxBytes, nil
			} else if quotaType == MaxPending {
				return q.MaxPending, nil
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
	}
	return 0, errors.New("no default for quota type - contact developer")
}
