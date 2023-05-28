package upload

import (
	"errors"
	"time"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/redislib"
)

func LockForUpload(ctx rcontext.RequestContext, hash string) (func() error, error) {
	mutex := redislib.GetMutex(hash, 5*time.Minute)
	if mutex != nil {
		if err := mutex.LockContext(ctx.Context); err != nil {
			return nil, errors.New("failed to acquire upload lock: " + err.Error())
		}
		return func() error {
			b, err := mutex.UnlockContext(ctx.Context)
			if !b {
				ctx.Log.Warn("Did not get quorum on unlock")
			}
			return err
		}, nil
	} else {
		ctx.Log.Warn("Continuing upload without lock! Set up Redis to make this warning go away.")
		return func() error { return nil }, nil
	}
}
