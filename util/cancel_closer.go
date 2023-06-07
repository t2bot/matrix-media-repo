package util

import "io"

type CancelCloser struct {
	io.ReadCloser
	r      io.ReadCloser
	cancel func()
}

func NewCancelCloser(r io.ReadCloser, cancel func()) *CancelCloser {
	return &CancelCloser{
		r:      r,
		cancel: cancel,
	}
}

func (c *CancelCloser) Read(p []byte) (int, error) {
	return c.r.Read(p)
}

func (c *CancelCloser) Close() error {
	c.cancel()
	return c.r.Close()
}
