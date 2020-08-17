package internal_cache

import (
	"bytes"
	"io"
	"io/ioutil"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/metrics"
	"github.com/turt2live/matrix-media-repo/redis_cache"
	"github.com/turt2live/matrix-media-repo/util/util_byte_seeker"
)

type RedisCache struct {
	redis *redis_cache.RedisCache
}

func NewRedisCache() *RedisCache {
	return &RedisCache{redis: redis_cache.NewCache()}
}

func (c *RedisCache) Reset() {
	// No-op
}

func (c *RedisCache) Stop() {
	_ = c.redis.Close()
}

func (c *RedisCache) MarkDownload(fileHash string) {
	// No-op
}

func (c *RedisCache) GetMedia(sha256hash string, contents FetchFunction, ctx rcontext.RequestContext) (*CachedContent, error) {
	return c.updateItemInCache(sha256hash, contents, ctx)
}

func (c *RedisCache) updateItemInCache(sha256hash string, fetchFn FetchFunction, ctx rcontext.RequestContext) (*CachedContent, error) {
	b, err := c.redis.GetBytes(ctx, sha256hash)
	if err == redis_cache.ErrCacheMiss || err == redis_cache.ErrCacheDown {
		metrics.CacheMisses.With(prometheus.Labels{"cache": "media"}).Inc()
		s, err := fetchFn()
		if err != nil {
			return nil, err
		}
		defer s.Close()
		fb, err := ioutil.ReadAll(s)
		if err != nil {
			return nil, err
		}
		err = c.redis.SetStream(ctx, sha256hash, bytes.NewReader(fb))
		if err != nil && err != redis_cache.ErrCacheDown {
			return nil, err
		}

		metrics.CacheHits.With(prometheus.Labels{"cache": "media"}).Inc()
		return &CachedContent{Contents: util_byte_seeker.NewByteSeeker(fb)}, nil
	}

	metrics.CacheHits.With(prometheus.Labels{"cache": "media"}).Inc()
	return &CachedContent{Contents: util_byte_seeker.NewByteSeeker(b)}, nil
}

func (c *RedisCache) UploadMedia(sha256hash string, content io.ReadCloser, ctx rcontext.RequestContext) error {
	defer content.Close()
	return c.redis.SetStream(ctx, sha256hash, content)
}
