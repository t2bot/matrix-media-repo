package redislib

import (
	"time"

	"github.com/go-redsync/redsync/v4"
)

func GetMutex(key string, expiration time.Duration) *redsync.Mutex {
	makeConnection()
	if rs == nil {
		return nil
	}

	return rs.NewMutex(key, redsync.WithExpiry(expiration))
}
