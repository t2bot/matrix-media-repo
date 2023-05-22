package quota

import (
	"errors"

	"github.com/ryanuber/go-glob"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
)

type Type int64

const (
	MaxBytes   Type = 0
	MaxPending Type = 1
)

var ErrQuotaExceeded = errors.New("quota exceeded")

func Check(ctx rcontext.RequestContext, userId string, quotaType Type) error {
	if !ctx.Config.Uploads.Quota.Enabled {
		return checkDefault(ctx, userId, quotaType)
	}

	for _, q := range ctx.Config.Uploads.Quota.UserQuotas {
		if glob.Glob(q.Glob, userId) {
			if quotaType == MaxBytes {
				if q.MaxBytes == 0 {
					return nil
				}
				total, err := database.GetInstance().UserStats.Prepare(ctx).UserUploadedBytes(userId)
				if err != nil {
					return err
				}
				if total >= q.MaxBytes {
					return ErrQuotaExceeded
				}
				return nil
			} else if quotaType == MaxPending {
				count, err := database.GetInstance().ExpiringMedia.Prepare(ctx).ByUserCount(userId)
				if err != nil {
					return err
				}
				if count < ctx.Config.Uploads.MaxPending {
					return nil
				}
				return ErrQuotaExceeded
			} else {
				return errors.New("no default for quota type - contact developer")
			}
		}
	}

	return checkDefault(ctx, userId, quotaType)
}

func checkDefault(ctx rcontext.RequestContext, userId string, quotaType Type) error {
	if quotaType == MaxBytes {
		return nil
	} else if quotaType == MaxPending {
		count, err := database.GetInstance().ExpiringMedia.Prepare(ctx).ByUserCount(userId)
		if err != nil {
			return err
		}
		if count < ctx.Config.Uploads.MaxPending {
			return nil
		}
		return ErrQuotaExceeded
	}

	return errors.New("no default for quota type - contact developer")
}
