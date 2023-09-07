package readers

import "io"

type nopSeekCloser struct {
	io.ReadSeeker
}

func (r nopSeekCloser) Close() error {
	return nil
}

func NopSeekCloser(r io.ReadSeeker) io.ReadSeekCloser {
	return nopSeekCloser{ReadSeeker: r}
}
