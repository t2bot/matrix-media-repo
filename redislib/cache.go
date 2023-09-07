package redislib

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/metrics"
)

const appendBufferSize = 32 * 1024 // 32kb
const mediaExpirationTime = 15 * time.Minute
const redisMaxValueSize = 512 * 1024 * 1024 // 512mb

func StoreMedia(ctx rcontext.RequestContext, hash string, content io.Reader, size int64) error {
	makeConnection()
	if ring == nil {
		return nil
	}
	if size >= redisMaxValueSize {
		ctx.Log.Debugf("Not caching %s because %d>%d", hash, size, redisMaxValueSize)
		return nil
	}

	if err := ring.ForEachShard(ctx.Context, func(ctx2 context.Context, client *redis.Client) error {
		res := client.Set(ctx2, hash, make([]byte, 0), mediaExpirationTime)
		return res.Err()
	}); err != nil {
		if delErr := DeleteMedia(ctx, hash); delErr != nil {
			ctx.Log.Warn("Error while attempting to clean up cache during another error: ", delErr)
			sentry.CaptureException(delErr)
		}
		return err
	}

	buf := make([]byte, appendBufferSize)
	for {
		read, err := content.Read(buf)
		eof := errors.Is(err, io.EOF)
		if read > 0 {
			if err = ring.ForEachShard(ctx.Context, func(ctx2 context.Context, client *redis.Client) error {
				res := client.Append(ctx2, hash, string(buf[0:read]))
				return res.Err()
			}); err != nil {
				if delErr := DeleteMedia(ctx, hash); delErr != nil {
					ctx.Log.Warn("Error while attempting to clean up cache during another error: ", delErr)
					sentry.CaptureException(delErr)
				}
				return err
			}
		}
		if eof {
			break
		}
	}

	return nil
}

func TryGetMedia(ctx rcontext.RequestContext, hash string) (io.Reader, error) {
	makeConnection()
	if ring == nil {
		return nil, nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx.Context, 20*time.Second)
	defer cancel()

	var result *redis.StringCmd

	// TODO(TR-1): @@ Return seekable stream
	ctx.Log.Debugf("Getting whole cached object for %s", hash)
	result = ring.Get(timeoutCtx, hash)

	s, err := result.Bytes()
	if err != nil {
		if err == redis.Nil {
			metrics.CacheMisses.With(prometheus.Labels{"cache": "media"}).Inc()
			return nil, nil
		}
		return nil, err
	}

	metrics.CacheHits.With(prometheus.Labels{"cache": "media"}).Inc()
	return bytes.NewBuffer(s), nil
}

func DeleteMedia(ctx rcontext.RequestContext, hash string) error {
	makeConnection()
	if ring == nil {
		return nil
	}

	return ring.ForEachShard(ctx, func(ctx2 context.Context, client *redis.Client) error {
		return client.Del(ctx2, hash).Err()
	})
}
