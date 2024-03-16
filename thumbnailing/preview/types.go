package preview

import (
	"io"
	"time"
)

type AudioInfo struct {
	KeySamples   [][2]float64
	Duration     time.Duration
	TotalSamples int
	Channels     int
}

type Thumbnail struct {
	Animated    bool
	ContentType string
	Reader      io.ReadCloser
}
