package m

import "io"

type PreviewResult struct {
	Url         string
	SiteName    string
	Type        string
	Description string
	Title       string
	Image       *PreviewImage
}

type PreviewImage struct {
	ContentType string
	Data        io.ReadCloser
	Filename    string
}
