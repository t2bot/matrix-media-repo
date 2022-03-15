package util

import (
	"sync"
	"time"
)

type mediaSet map[chan struct{}]struct{}

var waiterLock = &sync.Mutex{}
var waiters = map[string]mediaSet{}

func WaitForUpload(origin string, mediaId string, timeout time.Duration) bool {
	key := origin + mediaId
	ch := make(chan struct{}, 1)

	waiterLock.Lock()
	var set mediaSet
	var ok bool
	if set, ok = waiters[key]; !ok {
		set = make(mediaSet)
		waiters[key] = set
	}
	set[ch] = struct{}{}
	waiterLock.Unlock()

	defer func() {
		waiterLock.Lock()

		delete(set, ch)
		close(ch)

		if len(set) == 0 {
			delete(waiters, key)
		}

		waiterLock.Unlock()
	}()

	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}

func NotifyUpload(origin string, mediaId string) {
	waiterLock.Lock()
	defer waiterLock.Unlock()

	set := waiters[origin+mediaId]

	if set == nil {
		return
	}

	for channel := range set {
		channel <- struct{}{}
	}
}
