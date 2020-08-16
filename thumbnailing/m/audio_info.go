package m

import (
	"time"
)

type AudioInfo struct {
	KeySamples   [][2]float64
	Duration     time.Duration
	TotalSamples int
	Channels     int
}
