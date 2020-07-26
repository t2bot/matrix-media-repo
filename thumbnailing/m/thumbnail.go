package m

import (
	"io"
)

type Thumbnail struct {
	Animated    bool
	ContentType string
	Reader      io.ReadCloser
}
