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

	// Dev note: the prefix is to prevent key conflicts. Specifically, we create an upload mutex using
	// the sha256 hash of the file *and* populate the redis cache with that file at the same key - this
	// causes the mutex lock to fail unlocking because the value "changed". A prefix avoids that conflict.
	return rs.NewMutex("mutex-"+key, redsync.WithExpiry(expiration))
}
