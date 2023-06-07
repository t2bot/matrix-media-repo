package util

import (
	"strconv"
	"time"
)

func NowMillis() int64 {
	return time.Now().UnixNano() / 1000000
}

func FromMillis(m int64) time.Time {
	return time.Unix(0, m*int64(time.Millisecond))
}

func CalcBlockForDuration(timeoutMs string) (time.Duration, error) {
	blockFor := 20 * time.Second
	if timeoutMs != "" {
		parsed, err := strconv.Atoi(timeoutMs)
		if err != nil {
			return 0, err
		}
		if parsed > 0 {
			// Limit to 60 seconds
			if parsed > 60000 {
				parsed = 60000
			}
			blockFor = time.Duration(parsed) * time.Millisecond
		}
	}
	return blockFor, nil
}
