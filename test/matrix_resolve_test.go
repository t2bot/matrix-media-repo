package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t2bot/matrix-media-repo/matrix"
)

func doResolve(t *testing.T, origin string, expectedAddress string, expectedHost string) {
	url, host, err := matrix.GetServerApiUrl(origin)
	assert.NoError(t, err, origin)
	assert.Equal(t, expectedAddress, url, origin)
	assert.Equal(t, expectedHost, host, origin)
}

func TestResolveMatrix(t *testing.T) {
	doResolve(t, "2.s.resolvematrix.dev:7652", "https://2.s.resolvematrix.dev:7652", "2.s.resolvematrix.dev")
	doResolve(t, "3b.s.resolvematrix.dev", "https://wk.3b.s.resolvematrix.dev:7753", "wk.3b.s.resolvematrix.dev:7753")
	doResolve(t, "3c.s.resolvematrix.dev", "https://srv.wk.3c.s.resolvematrix.dev:7754", "wk.3c.s.resolvematrix.dev")
	doResolve(t, "3d.s.resolvematrix.dev", "https://wk.3d.s.resolvematrix.dev:8448", "wk.3d.s.resolvematrix.dev")
	doResolve(t, "4.s.resolvematrix.dev", "https://srv.4.s.resolvematrix.dev:7855", "4.s.resolvematrix.dev")
	doResolve(t, "5.s.resolvematrix.dev", "https://5.s.resolvematrix.dev:8448", "5.s.resolvematrix.dev")
	doResolve(t, "3c.msc4040.s.resolvematrix.dev", "https://srv.wk.3c.msc4040.s.resolvematrix.dev:7053", "wk.3c.msc4040.s.resolvematrix.dev")
	doResolve(t, "4.msc4040.s.resolvematrix.dev", "https://srv.4.msc4040.s.resolvematrix.dev:7054", "4.msc4040.s.resolvematrix.dev")
}
