package notifier

import (
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/redislib"
	"github.com/turt2live/matrix-media-repo/util"
)

var localWaiters = make(map[string][]chan *database.DbMedia)
var mu = new(sync.Mutex)
var redisChan <-chan string

const uploadsNotifyRedisChannel = "mmr:upload_mxc"

func GetUploadWaitChannel(origin string, mediaId string) (<-chan *database.DbMedia, func()) {
	subscribeRedis()
	mxc := util.MxcUri(origin, mediaId)

	mu.Lock()
	defer mu.Unlock()

	if _, ok := localWaiters[mxc]; !ok {
		localWaiters[mxc] = make([]chan *database.DbMedia, 0)
	}

	ch := make(chan *database.DbMedia)
	localWaiters[mxc] = append(localWaiters[mxc], ch)

	finishFn := func() {
		mu.Lock()
		defer mu.Unlock()

		if arr, ok := localWaiters[mxc]; ok {
			newArr := make([]chan *database.DbMedia, 0)
			for _, xch := range arr {
				if xch != ch {
					newArr = append(newArr, xch)
				}
			}
			localWaiters[mxc] = newArr
		}

		close(ch)
	}

	return ch, finishFn
}

func UploadDone(ctx rcontext.RequestContext, record *database.DbMedia) error {
	mxc := util.MxcUri(record.Origin, record.MediaId)
	noRelayNotifyUpload(record)
	return redislib.Publish(ctx, uploadsNotifyRedisChannel, mxc)
}

func noRelayNotifyUpload(record *database.DbMedia) {
	go func() {
		mxc := util.MxcUri(record.Origin, record.MediaId)

		mu.Lock()
		defer mu.Unlock()

		if arr, ok := localWaiters[mxc]; ok {
			for _, ch := range arr {
				ch <- record
			}
			delete(localWaiters, mxc)
		}
	}()
}

func subscribeRedis() {
	if redisChan != nil {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	redisChan = redislib.Subscribe(uploadsNotifyRedisChannel)
	if redisChan == nil {
		return // no redis to subscribe with
	}
	go func() {
		for {
			val := <-redisChan
			logrus.Debug("Received value from uploads notify channel: ", val)

			origin, mediaId, err := util.SplitMxc(val)
			if err != nil {
				logrus.Warn("Non-fatal error receiving from uploads notify channel: ", err)
				continue
			}

			db := database.GetInstance().Media.Prepare(rcontext.Initial())
			record, err := db.GetById(origin, mediaId)
			if err != nil {
				logrus.Warn("Non-fatal error processing record from uploads notify channel: ", err)
				continue
			}
			if record == nil {
				logrus.Warn("Received notification that a media record is available, but it's not")
				continue
			}

			noRelayNotifyUpload(record)
		}
	}()
}
