package notifier

import (
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/redislib"
	"github.com/t2bot/matrix-media-repo/util"
)

var localUploadWaiters = make(map[string][]chan *database.DbMedia)
var uploadMutex = new(sync.Mutex)
var uploadsRedisChan <-chan string

const uploadsNotifyRedisChannel = "mmr:upload_mxc"

func GetUploadWaitChannel(origin string, mediaId string) (<-chan *database.DbMedia, func()) {
	subscribeRedisUploads()
	mxc := util.MxcUri(origin, mediaId)

	uploadMutex.Lock()
	defer uploadMutex.Unlock()

	if _, ok := localUploadWaiters[mxc]; !ok {
		localUploadWaiters[mxc] = make([]chan *database.DbMedia, 0)
	}

	ch := make(chan *database.DbMedia, 1)
	localUploadWaiters[mxc] = append(localUploadWaiters[mxc], ch)

	finishFn := func() {
		uploadMutex.Lock()
		defer uploadMutex.Unlock()
		defer close(ch)

		if arr, ok := localUploadWaiters[mxc]; ok {
			newArr := make([]chan *database.DbMedia, 0)
			for _, xch := range arr {
				if xch != ch {
					newArr = append(newArr, xch)
				}
			}
			localUploadWaiters[mxc] = newArr
		}
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

		uploadMutex.Lock()
		defer uploadMutex.Unlock()

		if arr, ok := localUploadWaiters[mxc]; ok {
			for _, ch := range arr {
				ch <- record
			}
			delete(localUploadWaiters, mxc)
		}
	}()
}

func subscribeRedisUploads() {
	if uploadsRedisChan != nil {
		return
	}

	uploadMutex.Lock()
	defer uploadMutex.Unlock()

	uploadsRedisChan = redislib.Subscribe(uploadsNotifyRedisChannel)
	if uploadsRedisChan == nil {
		return // no redis to subscribe with
	}
	go func() {
		for {
			val := <-uploadsRedisChan
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
