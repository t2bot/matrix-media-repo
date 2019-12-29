package preview_types

import (
	"errors"
	"io"
	"net/url"
)

type PreviewResult struct {
	Url         string
	SiteName    string
	Type        string
	Description string
	Title       string
	Image       *PreviewImage
}

type PreviewImage struct {
	ContentType         string
	Data                io.ReadCloser
	Filename            string
	ContentLength       int64
	ContentLengthHeader string
}

type UrlPayload struct {
	UrlString string
	ParsedUrl *url.URL
}

var ErrPreviewUnsupported = errors.New("preview not supported by this previewer")
