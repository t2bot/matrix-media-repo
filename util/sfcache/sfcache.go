package sfcache

import (
	"sync"

	"github.com/t2bot/go-typed-singleflight"
)

type SingleflightCache[T comparable] struct {
	sf    *typedsf.Group[T]
	cache *sync.Map
}

func NewSingleflightCache[T comparable]() *SingleflightCache[T] {
	return &SingleflightCache[T]{
		sf:    new(typedsf.Group[T]),
		cache: new(sync.Map),
	}
}

func (c *SingleflightCache[T]) Do(key string, fn func() (T, error)) (T, error) {
	if v, ok := c.cache.Load(key); ok {
		// Safe cast because incorrect types are filtered out before storage
		return v.(T), nil
	}
	var zero T
	v, err, _ := c.sf.Do(key, fn)
	if err == nil && v != zero {
		c.cache.Store(key, v)
	}
	return v, err
}

func (c *SingleflightCache[T]) OverwriteCacheKey(key string, val T) {
	c.cache.Store(key, val)
}

func (c *SingleflightCache[T]) ForgetCacheKey(key string) {
	c.cache.Delete(key)
}
