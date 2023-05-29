package upload

import (
	"errors"
	"time"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/redislib"
)

const maxLockAttemptTime = 30 * time.Second

func LockForUpload(ctx rcontext.RequestContext, hash string) (func() error, error) {
	mutex := redislib.GetMutex(hash, 1*time.Minute)
	if mutex != nil {
		attemptDoneAt := time.Now().Add(maxLockAttemptTime)
		acquired := false
		for !acquired {
			if chErr := ctx.Context.Err(); chErr != nil {
				return nil, chErr
			}
			if err := mutex.LockContext(ctx.Context); err != nil {
				ctx.Log.Warn("failed to acquire upload lock")
				if time.Now().After(attemptDoneAt) {
					return nil, errors.New("failed to acquire upload lock: " + err.Error())
				}
			} else {
				acquired = true
			}
		}
		if !acquired {
			return nil, errors.New("failed to acquire upload lock: timeout")
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
