package resource_handler

import (
	"reflect"
	"time"

	"github.com/Jeffail/tunny"
	"github.com/olebedev/emitter"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
)

type ResourceHandler struct {
	pool      *tunny.Pool
	eventBus  *emitter.Emitter
	itemCache *cache.Cache
}

type resource struct {
	isComplete bool
	result     interface{}
}

type WorkRequest struct {
	Id       string
	Metadata interface{}
}

func New(workers int, fetchFn func(object *WorkRequest) interface{}) (*ResourceHandler, error) {
	workFn := func(i interface{}) interface{} { return fetchFn(i.(*WorkRequest)) }
	pool := tunny.NewFunc(workers, workFn)

	bus := &emitter.Emitter{}
	itemCache := cache.New(30*time.Second, 1*time.Minute) // cache work for 30ish seconds

	handler := &ResourceHandler{pool, bus, itemCache}
	return handler, nil
}

func (h *ResourceHandler) Close() {
	logrus.Warn("Closing resource handler: " + reflect.TypeOf(h).Name())
	h.pool.Close()
}

func (h *ResourceHandler) GetResource(id string, metadata interface{}) chan interface{} {
	resultChan := make(chan interface{})

	// First see if we have already cached this request
	cachedResource, found := h.itemCache.Get(id)
	if found {
		res := cachedResource.(*resource)

		// If the request has already been completed, return that result
		if res.isComplete {
			// This is a goroutine to avoid a problem where the sending and return can race
			go func() {
				logrus.Warn("Returning cached reply from resource handler for resource ID " + id)
				resultChan <- res.result
			}()
			return resultChan
		}

		// Otherwise queue a wait function to handle the resource when it is complete
		go func() {
			result := <-h.eventBus.Once("complete_" + id)
			resultChan <- result.Args[0]
		}()

		return resultChan
	}

	// Cache that we're starting the request (never expire)
	h.itemCache.Set(id, &resource{false, nil}, cache.NoExpiration)

	go func() {
		// Queue the work (ignore errors)
		result := h.pool.Process(&WorkRequest{id, metadata})
		h.eventBus.Emit("complete_"+id, result)

		// Cache the result for future callers
		newResource := &resource{
			isComplete: true,
			result:     result,
		}
		h.itemCache.Set(id, newResource, cache.DefaultExpiration)

		// and finally feed it back to the caller
		resultChan <- result
	}()

	return resultChan
}
