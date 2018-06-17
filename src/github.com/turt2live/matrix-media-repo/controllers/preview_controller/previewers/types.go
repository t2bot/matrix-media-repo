package previewers

import (
	"errors"
	"io"
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

var ErrPreviewUnsupported = errors.New("preview not supported by this previewer")
