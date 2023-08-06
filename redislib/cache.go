package redislib

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/metrics"
)

const appendBufferSize = 1024 // 1kb
const mediaExpirationTime = 5 * time.Minute
const redisMaxValueSize = 512 * 1024 * 1024 // 512mb

func StoreMedia(ctx rcontext.RequestContext, hash string, content io.Reader, size int64) error {
	makeConnection()
	if ring == nil {
		return nil
	}
	if size >= redisMaxValueSize {
		return nil
	}

	if err := ring.ForEachShard(ctx.Context, func(ctx2 context.Context, client *redis.Client) error {
		res := client.Set(ctx2, hash, make([]byte, 0), mediaExpirationTime)
		return res.Err()
	}); err != nil {
		return err
	}

	buf := make([]byte, appendBufferSize)
	for {
		read, err := content.Read(buf)
		if err == io.EOF {
			break
		}
		if err = ring.ForEachShard(ctx.Context, func(ctx2 context.Context, client *redis.Client) error {
			res := client.Append(ctx2, hash, string(buf[0:read]))
			return res.Err()
		}); err != nil {
			return err
		}
	}

	return nil
}

func TryGetMedia(ctx rcontext.RequestContext, hash string, startByte int64, endByte int64) (io.Reader, error) {
	makeConnection()
	if ring == nil {
		return nil, nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx.Context, 20*time.Second)
	defer cancel()

	var result *redis.StringCmd
	if startByte >= 0 && endByte >= 1 {
		if startByte < endByte {
			result = ring.GetRange(timeoutCtx, hash, startByte, endByte)
		} else {
			return nil, errors.New("invalid range - start must be before end")
		}
	} else {
		result = ring.Get(timeoutCtx, hash)
	}

	s, err := result.Result()
	if err != nil {
		if err == redis.Nil {
			metrics.CacheMisses.With(prometheus.Labels{"cache": "media"}).Inc()
			return nil, nil
		}
		return nil, err
	}

	metrics.CacheHits.With(prometheus.Labels{"cache": "media"}).Inc()
	return bytes.NewBuffer([]byte(s)), nil
}

func DeleteMedia(ctx rcontext.RequestContext, hash string) {
	makeConnection()
	if ring == nil {
		return
	}

	ring.Del(ctx, hash)
}
