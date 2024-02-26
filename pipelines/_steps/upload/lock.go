package upload

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/redislib"
)

const maxLockAttemptTime = 30 * time.Second

func LockForUpload(ctx rcontext.RequestContext, hash string) (func() error, error) {
	mutex := redislib.GetMutex(hash, 5*time.Minute)
	if mutex != nil {
		attemptDoneAt := time.Now().Add(maxLockAttemptTime)
		acquired := false
		for !acquired {
			if chErr := ctx.Context.Err(); chErr != nil {
				return nil, chErr
			}
			if err := mutex.LockContext(ctx.Context); err != nil {
				if time.Now().After(attemptDoneAt) {
					return nil, fmt.Errorf("failed to acquire upload lock: %w", err)
				} else {
					ctx.Log.Warn("failed to acquire upload lock: ", err)
				}
			} else {
				acquired = true
			}
		}
		if !acquired {
			return nil, errors.New("failed to acquire upload lock: timeout")
		}
		ctx.Log.Debugf("Lock acquired until %s", mutex.Until().UTC())
		return func() error {
			ctx.Log.Debug("Unlocking upload lock")
			// We use a background context here to prevent a cancelled context from keeping the lock open
			if ok, err := mutex.UnlockContext(context.Background()); !ok || err != nil {
				ctx.Log.Warn("Did not get quorum on unlock: ", err)
				return err
			}
			return nil
		}, nil
	} else {
		ctx.Log.Warn("Continuing upload without lock! Set up Redis to make this warning go away.")
		return func() error { return nil }, nil
	}
}
