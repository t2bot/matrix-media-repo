package tasks

import (
	"math/rand"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/controllers/maintenance_controller"
	"github.com/turt2live/matrix-media-repo/util"
)

var mediaPurgeDone chan bool

func StartRemoteMediaPurgeRecurring() {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	ticker := time.NewTicker((1 * time.Hour) + (time.Duration(r.Intn(15)) * time.Minute))
	mediaPurgeDone = make(chan bool)

	go func() {
		for {
			select {
			case <-mediaPurgeDone:
				ticker.Stop()
				return
			case <-ticker.C:
				if config.Get().Downloads.ExpireDays <= 0 {
					continue
				}

				doRecurringRemoteMediaPurge()
			}
		}
	}()
}

func StopRemoteMediaPurgeRecurring() {
	mediaPurgeDone <- true
}

func doRecurringRemoteMediaPurge() {
	ctx := rcontext.Initial().LogWithFields(logrus.Fields{"task": "recurring_purge_remote_media"})
	ctx.Log.Info("Starting remote media purge task")

	// We get media that is N days old to make sure it gets cleared safely.
	beforeTs := util.NowMillis() - int64(config.Get().Downloads.ExpireDays*24*60*60*1000)

	_, err := maintenance_controller.PurgeRemoteMediaBefore(beforeTs, ctx)
	if err != nil {
		ctx.Log.Error(err)
	}
	ctx.Log.Info("Purge task completed")
}
