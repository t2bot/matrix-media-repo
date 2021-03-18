package tasks

import (
	"github.com/getsentry/sentry-go"
	"math/rand"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/util"
)

var previewsPurgeDone chan bool

func StartPreviewsPurgeRecurring() {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	ticker := time.NewTicker((1 * time.Hour) + (time.Duration(r.Intn(15)) * time.Minute))
	previewsPurgeDone = make(chan bool)

	go func() {
		defer close(previewsPurgeDone)
		for {
			select {
			case <-previewsPurgeDone:
				ticker.Stop()
				return
			case <-ticker.C:
				if config.Get().UrlPreviews.ExpireDays <= 0 {
					continue
				}

				doRecurringPreviewPurge()
			}
		}
	}()
}

func StopPreviewsPurgeRecurring() {
	previewsPurgeDone <- true
}

func doRecurringPreviewPurge() {
	ctx := rcontext.Initial().LogWithFields(logrus.Fields{"task": "recurring_purge_url_previews"})
	ctx.Log.Info("Starting URL preview purge task")

	// We get media that is N days old to make sure it gets cleared safely.
	beforeTs := util.NowMillis() - int64(config.Get().UrlPreviews.ExpireDays*24*60*60*1000)

	db := storage.GetDatabase().GetUrlStore(ctx)
	err := db.DeleteOlderThan(beforeTs)
	if err != nil {
		ctx.Log.Error(err)
		sentry.CaptureException(err)
	}
	ctx.Log.Info("Purge task completed")
}
