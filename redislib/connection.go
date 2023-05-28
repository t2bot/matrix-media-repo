package redislib

import (
	"sync"
	"time"

	"github.com/go-redsync/redsync/v4"
	rsredis "github.com/go-redsync/redsync/v4/redis"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/redis/go-redis/v9"
	"github.com/turt2live/matrix-media-repo/common/config"
)

var connectionLock = &sync.Once{}
var ring *redis.Ring
var rs *redsync.Redsync
var pools = make([]rsredis.Pool, 0)
var clients = make([]*redis.Client, 0)

func makeConnection() {
	if ring != nil {
		return
	}

	connectionLock.Do(func() {
		conf := config.Get().Redis
		if !conf.Enabled {
			return
		}
		addresses := make(map[string]string)
		for _, c := range conf.Shards {
			addresses[c.Name] = c.Address

			client := redis.NewClient(&redis.Options{
				DialTimeout: 10 * time.Second,
				DB:          conf.DbNum,
				Addr:        c.Address,
			})
			clients = append(clients, client)
			pools = append(pools, goredis.NewPool(client))
		}
		ring = redis.NewRing(&redis.RingOptions{
			Addrs:       addresses,
			DialTimeout: 10 * time.Second,
			DB:          conf.DbNum,
		})
		rs = redsync.New(pools...)
	})
}

func Reconnect() {
	Stop()
	makeConnection()
}

func Stop() {
	if ring != nil {
		_ = ring.Close()
	}
	for _, c := range clients {
		_ = c.Close()
	}
	ring = nil
	rs = nil
	pools = make([]rsredis.Pool, 0)
	clients = make([]*redis.Client, 0)
	connectionLock = &sync.Once{}
}
