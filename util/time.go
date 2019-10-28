package util

import "time"

func NowMillis() int64 {
	return time.Now().UnixNano() / 1000000
}

func FromMillis(m int64) time.Time {
	return time.Unix(0, m*int64(time.Millisecond))
}
