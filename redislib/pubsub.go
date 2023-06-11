package redislib

import (
	"context"
	"sync"

	"github.com/redis/go-redis/v9"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

var subscribeMutex = new(sync.Mutex)
var subscribeChans = make(map[string][]chan string)

type PubSubValue struct {
	Err error
	Str string
}

func Publish(ctx rcontext.RequestContext, channel string, payload string) error {
	makeConnection()
	if ring == nil {
		return nil
	}

	if ring.PoolStats().TotalConns == 0 {
		ctx.Log.Warn("Not broadcasting upload to Redis - no connections available")
		return nil
	}

	r := ring.Publish(ctx.Context, channel, payload)
	if r.Err() != nil {
		if r.Err() == redis.Nil {
			ctx.Log.Warn("Not broadcasting upload to Redis - no connections available")
			return nil
		}
		return r.Err()
	}
	return nil
}

func Subscribe(channel string) <-chan string {
	makeConnection()
	if ring == nil {
		return nil
	}

	ch := make(chan string)
	subscribeMutex.Lock()
	if _, ok := subscribeChans[channel]; !ok {
		subscribeChans[channel] = make([]chan string, 0)
	}
	subscribeChans[channel] = append(subscribeChans[channel], ch)
	subscribeMutex.Unlock()
	doSubscribe(channel, ch)
	return ch
}

func doSubscribe(channel string, ch chan<- string) {
	sub := ring.Subscribe(context.Background(), channel)
	go func(ch chan<- string) {
		recvCh := sub.Channel()
		for {
			val := <-recvCh
			if val != nil {
				ch <- val.Payload
			} else {
				break
			}
		}
	}(ch)
}

func resubscribeAll() {
	subscribeMutex.Lock()
	defer subscribeMutex.Unlock()
	for channel, chs := range subscribeChans {
		for _, ch := range chs {
			if ring == nil {
				close(ch)
			} else {
				doSubscribe(channel, ch)
			}
		}
	}
	if ring == nil {
		subscribeChans = make(map[string][]chan string)
	}
}
