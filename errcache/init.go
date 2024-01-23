package errcache

import (
	"time"

	"github.com/t2bot/matrix-media-repo/common/config"
)

var DownloadErrors *ErrCache

func Init() {
	DownloadErrors = NewErrCache(time.Duration(config.Get().Downloads.FailureCacheMinutes) * time.Minute)
}

func AdjustSize() {
	DownloadErrors.Resize(time.Duration(config.Get().Downloads.FailureCacheMinutes) * time.Minute)
}
