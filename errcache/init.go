package errcache

import (
	"time"

	"github.com/turt2live/matrix-media-repo/common/config"
)

var DownloadErrors *ErrCache

func Init() {
	DownloadErrors = NewErrCache(time.Duration(config.Get().Downloads.FailureCacheMinutes) * time.Minute)
}

func AdjustSize() {
	DownloadErrors.Resize(time.Duration(config.Get().Downloads.FailureCacheMinutes) * time.Minute)
}
