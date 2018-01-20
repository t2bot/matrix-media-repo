package types

import "io"

type Thumbnail struct {
	Origin      string
	MediaId     string
	Width       int
	Height      int
	Method      string // "crop" or "scale"
	Animated    bool
	ContentType string
	SizeBytes   int64
	Location    string
	CreationTs  int64
}

type StreamedThumbnail struct {
	Thumbnail *Thumbnail
	Stream    io.ReadCloser
}
