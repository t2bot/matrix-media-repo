package internal_cache

import (
	"io"

	"github.com/turt2live/matrix-media-repo/common/rcontext"
)

type CachedContent struct {
	Contents io.ReadSeeker
}

type FetchFunction func() (io.ReadCloser, error)

type ContentCache interface {
	Reset()
	Stop()
	MarkDownload(fileHash string)
	GetMedia(sha256hash string, contents FetchFunction, ctx rcontext.RequestContext) (*CachedContent, error)
	UploadMedia(sha256hash string, content io.ReadCloser, ctx rcontext.RequestContext) error
}
