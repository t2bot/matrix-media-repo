package pool

import (
	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common/config"
)

var DownloadQueue *Queue
var ThumbnailQueue *Queue
var UrlPreviewQueue *Queue
var TaskQueue *Queue

func Init() {
	var err error
	if DownloadQueue, err = NewQueue(config.Get().Downloads.NumWorkers, "downloads"); err != nil {
		sentry.CaptureException(err)
		logrus.Error("Error setting up downloads queue")
		logrus.Fatal(err)
	}
	if ThumbnailQueue, err = NewQueue(config.Get().Thumbnails.NumWorkers, "thumbnails"); err != nil {
		sentry.CaptureException(err)
		logrus.Error("Error setting up thumbnails queue")
		logrus.Fatal(err)
	}
	if UrlPreviewQueue, err = NewQueue(config.Get().UrlPreviews.NumWorkers, "url_previews"); err != nil {
		sentry.CaptureException(err)
		logrus.Error("Error setting up url previews queue")
		logrus.Fatal(err)
	}
	if TaskQueue, err = NewQueue(config.Get().Tasks.NumWorkers, "tasks"); err != nil {
		sentry.CaptureException(err)
		logrus.Error("Error setting up tasks queue")
		logrus.Fatal(err)
	}
}

func AdjustSize() {
	DownloadQueue.pool.Tune(config.Get().Downloads.NumWorkers)
	ThumbnailQueue.pool.Tune(config.Get().Thumbnails.NumWorkers)
	UrlPreviewQueue.pool.Tune(config.Get().UrlPreviews.NumWorkers)
	TaskQueue.pool.Tune(config.Get().Tasks.NumWorkers)
}

func Drain() {
	DownloadQueue.pool.Release()
	ThumbnailQueue.pool.Release()
	UrlPreviewQueue.pool.Release()
	TaskQueue.pool.Release()
}
