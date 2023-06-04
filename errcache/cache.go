package errcache

import (
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

type ErrCache struct {
	cache *cache.Cache
	mu    sync.Mutex
}

func NewErrCache(expiration time.Duration) *ErrCache {
	return &ErrCache{cache: cache.New(expiration, expiration*2)}
}

func (e *ErrCache) Resize(expiration time.Duration) {
	e.mu.Lock()
	e.cache = cache.NewFrom(expiration, expiration*2, e.cache.Items())
	e.mu.Unlock()
}

func (e *ErrCache) Get(key string) error {
	e.mu.Lock()
	if err, ok := e.cache.Get(key); ok {
		e.mu.Unlock()
		return err.(error)
	}
	e.mu.Unlock()
	return nil
}

func (e *ErrCache) Set(key string, err error) {
	e.mu.Lock()
	e.cache.Set(key, err, cache.DefaultExpiration)
	e.mu.Unlock()
}
