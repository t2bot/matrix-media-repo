package url_previewing

import (
	"errors"
	"io"
	"net/url"
)

type Result struct {
	Url         string
	SiteName    string
	Type        string
	Description string
	Title       string
	Image       *Image
}

type Image struct {
	ContentType string
	Data        io.ReadCloser
	Filename    string
}

type UrlPayload struct {
	UrlString string
	ParsedUrl *url.URL
}

var ErrPreviewUnsupported = errors.New("preview not supported by this previewer")
