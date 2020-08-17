package internal_cache

import (
	"io"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/metrics"
)

type NoopCache struct{}

func NewNoopCache() *NoopCache {
	return &NoopCache{}
}

func (n *NoopCache) Reset() {
	// do nothing
}

func (n *NoopCache) Stop() {
	// do nothing
}

func (n *NoopCache) MarkDownload(fileHash string) {
	// do nothing
}

func (n *NoopCache) GetMedia(sha256hash string, contents FetchFunction, ctx rcontext.RequestContext) (*CachedContent, error) {
	metrics.CacheMisses.With(prometheus.Labels{"cache": "media"}).Inc()
	return nil, nil
}

func (n *NoopCache) UploadMedia(sha256hash string, content io.ReadCloser, ctx rcontext.RequestContext) error {
	// do nothing
	return nil
}
