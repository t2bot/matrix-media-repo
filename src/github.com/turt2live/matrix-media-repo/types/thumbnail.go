package types

type Thumbnail struct {
	Origin      string
	MediaId     string
	Width       int
	Height      int
	Method      string // "crop" or "scale"
	ContentType string
	SizeBytes   int64
	Location    string
	CreationTs  int64
}
