package singleflight_counter

import (
	"sync"
)

// Largely inspired by Go's singleflight package.
// https://github.com/golang/sync/blob/112230192c580c3556b8cee6403af37a4fc5f28c/singleflight/singleflight.go

type call struct {
	wg sync.WaitGroup

	valsMu    sync.Mutex
	nextIndex int
	vals      []interface{}

	val   interface{}
	err   error
	count int
}

type Group struct {
	mu sync.Mutex
	m  map[string]*call
}

func (c *call) NextVal() interface{} {
	c.valsMu.Lock()
	val := c.val
	if c.vals != nil && len(c.vals) >= c.count {
		val = c.vals[c.nextIndex]
		c.nextIndex++
	}
	c.valsMu.Unlock()
	return val
}

func (g *Group) DoWithoutPost(key string, fn func() (interface{}, error)) (interface{}, int, error) {
	return g.Do(key, fn, func(v interface{}, total int, e error) []interface{} {
		return nil
	})
}

func (g *Group) Do(key string, fn func() (interface{}, error), postprocess func(v interface{}, total int, e error) []interface{}) (interface{}, int, error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		c.count++
		g.mu.Unlock()
		c.wg.Wait()

		return c.NextVal(), c.count, c.err
	}

	c := new(call)
	c.count = 1 // Always start at 1 (for us)
	c.nextIndex = 0
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	c.vals = nil
	if postprocess != nil {
		c.vals = postprocess(c.val, c.count, c.err)
	}

	c.wg.Done()

	return c.NextVal(), c.count, c.err
}
