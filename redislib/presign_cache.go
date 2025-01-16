package redislib

import (
	"context"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/redis/go-redis/v9"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
)

const keyPrefix = "s3url:"

func StoreURL(ctx rcontext.RequestContext, dsFileName string, url string, expiration time.Duration) error {
	makeConnection()
	if ring == nil {
		return nil
	}

	if err := ring.ForEachShard(ctx.Context, func(ctx2 context.Context, client *redis.Client) error {
		res := client.Set(ctx2, keyPrefix+dsFileName, url, expiration)
		return res.Err()
	}); err != nil {
		if delErr := DeleteURL(ctx, keyPrefix+dsFileName); delErr != nil {
			ctx.Log.Warn("Error while attempting to clean up url cache during another error: ", delErr)
			sentry.CaptureException(delErr)
		}
		return err
	}

	return nil
}

func TryGetURL(ctx rcontext.RequestContext, dsFileName string) (string, error) {
	makeConnection()
	if ring == nil {
		return "", nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx.Context, 20*time.Second)
	defer cancel()

	var result *redis.StringCmd

	ctx.Log.Debugf("Getting cached s3 url for %s", keyPrefix+dsFileName)
	result = ring.Get(timeoutCtx, keyPrefix+dsFileName)

	s, err := result.Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", err
	}

	return s, nil
}

func DeleteURL(ctx rcontext.RequestContext, dsFileName string) error {
	makeConnection()
	if ring == nil {
		return nil
	}

	return ring.ForEachShard(ctx, func(ctx2 context.Context, client *redis.Client) error {
		return client.Del(ctx2, keyPrefix+dsFileName).Err()
	})
}
