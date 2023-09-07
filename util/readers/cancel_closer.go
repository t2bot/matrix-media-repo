package readers

import "io"

type CancellableCloser interface {
	io.ReadCloser
}

type CancelCloser struct {
	io.ReadCloser
	cancel func()
}

type CancelSeekCloser struct {
	io.ReadSeekCloser
	cancel func()
}

func NewCancelCloser(r io.ReadCloser, cancel func()) CancellableCloser {
	if rsc, ok := r.(io.ReadSeekCloser); ok {
		return &CancelSeekCloser{
			ReadSeekCloser: rsc,
			cancel:         cancel,
		}
	} else {
		return &CancelCloser{
			ReadCloser: r,
			cancel:     cancel,
		}
	}
}

func (c *CancelCloser) Close() error {
	c.cancel()
	return c.ReadCloser.Close()
}

func (c *CancelSeekCloser) Close() error {
	c.cancel()
	return c.ReadSeekCloser.Close()
}
