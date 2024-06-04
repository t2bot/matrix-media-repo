package limits

import (
	"sync"
	"time"

	"github.com/t2bot/go-leaky-bucket"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

var buckets = make(map[string]*leaky.Bucket)
var bucketLock = &sync.Mutex{}

func GetBucket(ctx rcontext.RequestContext, subject string) (*leaky.Bucket, error) {
	if !config.Get().RateLimit.Enabled {
		return nil, nil
	}

	bucketLock.Lock()
	defer bucketLock.Unlock()

	bucket, ok := buckets[subject]
	if !ok {
		var err error
		bucket, err = leaky.NewBucket(config.Get().RateLimit.Buckets.Downloads.DrainBytesPerMinute, time.Minute, config.Get().RateLimit.Buckets.Downloads.CapacityBytes)
		if err != nil {
			return nil, err
		}
		bucket.OverflowLimit = config.Get().RateLimit.Buckets.Downloads.OverflowLimitBytes
		buckets[subject] = bucket
	}

	return bucket, nil
}

func ExpandBuckets() {
	bucketLock.Lock()
	defer bucketLock.Unlock()

	for _, bucket := range buckets {
		bucket.Capacity = config.Get().RateLimit.Buckets.Downloads.CapacityBytes
		bucket.DrainBy = config.Get().RateLimit.Buckets.Downloads.DrainBytesPerMinute
	}
}
