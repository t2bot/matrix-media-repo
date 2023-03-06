package redis_cache

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/go-redis/redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

var ErrCacheMiss = errors.New("missed cache")
var ErrCacheDown = errors.New("all shards appear to be down")

type RedisCache struct {
	ring *redis.Ring
}

func NewCache(conf config.RedisConfig) *RedisCache {
	addresses := make(map[string]string)
	for _, c := range conf.Shards {
		addresses[c.Name] = c.Address
	}
	ring := redis.NewRing(&redis.RingOptions{
		Addrs:       addresses,
		DialTimeout: 10 * time.Second,
		DB:          conf.DbNum,
	})

	logrus.Info("Contacting Redis shards...")
	_ = ring.ForEachShard(context.Background(), func(ctx context.Context, client *redis.Client) error {
		logrus.Infof("Pinging %s", client.String())
		r, err := client.Ping(ctx).Result()
		if err != nil {
			return err
		}
		logrus.Infof("%s replied with: %s", client.String(), r)
		return nil
	})

	return &RedisCache{ring: ring}
}

func (c *RedisCache) Close() error {
	return c.ring.Close()
}

func (c *RedisCache) SetStream(ctx rcontext.RequestContext, key string, s io.Reader) error {
	b, err := io.ReadAll(s)
	if err != nil {
		return err
	}
	return c.SetBytes(ctx, key, b)
}

func (c *RedisCache) GetStream(ctx rcontext.RequestContext, key string) (io.Reader, error) {
	b, err := c.GetBytes(ctx, key)
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(b), nil
}

func (c *RedisCache) SetBytes(ctx rcontext.RequestContext, key string, b []byte) error {
	if c.ring.PoolStats().TotalConns == 0 {
		return ErrCacheDown
	}
	_, err := c.ring.Set(ctx.Context, key, b, time.Duration(0)).Result() // no expiration (zero)
	if err != nil && c.ring.PoolStats().TotalConns == 0 {
		ctx.Log.Error(err)
		return ErrCacheDown
	}
	return err
}

func (c *RedisCache) GetBytes(ctx rcontext.RequestContext, key string) ([]byte, error) {
	if c.ring.PoolStats().TotalConns == 0 {
		return nil, ErrCacheDown
	}
	r := c.ring.Get(ctx.Context, key)
	if r.Err() != nil {
		if r.Err() == redis.Nil {
			return nil, ErrCacheMiss
		}
		if c.ring.PoolStats().TotalConns == 0 {
			ctx.Log.Error(r.Err())
			return nil, ErrCacheDown
		}
		return nil, r.Err()
	}

	b, err := r.Bytes()
	return b, err
}
