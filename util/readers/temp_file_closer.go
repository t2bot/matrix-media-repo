package readers

import (
	"io"
	"os"
)

type TempFileCloser struct {
	io.ReadSeekCloser
	fname    string
	fpath    string
	upstream io.ReadSeekCloser
	closed   bool
}

func NewTempFileCloser(fpath string, fname string, upstream io.ReadSeekCloser) *TempFileCloser {
	return &TempFileCloser{
		fname:    fname,
		fpath:    fpath,
		upstream: upstream,
		closed:   false,
	}
}

func (c *TempFileCloser) Close() error {
	if c.closed {
		return nil
	}

	upstreamErr := c.upstream.Close()
	// don't return upstreamErr yet because we want to try to delete the temp file

	var err error
	if err = os.Remove(c.fname); err != nil && !os.IsNotExist(err) {
		return err
	}
	if c.fpath != "" {
		if err = os.Remove(c.fpath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	c.closed = true
	return upstreamErr
}

func (c *TempFileCloser) Read(p []byte) (n int, err error) {
	return c.upstream.Read(p)
}

func (c *TempFileCloser) Seek(offset int64, whence int) (int64, error) {
	return c.upstream.Seek(offset, whence)
}
