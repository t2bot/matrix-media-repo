package readers

import (
	"io"

	"github.com/turt2live/matrix-media-repo/common"
)

func LimitReaderWithOverrunError(r io.ReadCloser, n int64) io.ReadCloser {
	return &limitedReader{r: r, n: n}
}

type limitedReader struct {
	r io.ReadCloser
	n int64
}

func (r *limitedReader) Read(p []byte) (int, error) {
	if r.n <= 0 {
		// See if we can read one more byte, indicating the stream is too big
		b := make([]byte, 1)
		n, err := r.r.Read(b)
		p[0] = b[0]
		if err != nil {
			// ignore - we're at the end anyways
			return n, io.EOF
		}
		if n > 0 {
			return n, common.ErrMediaTooLarge
		}

		return n, io.EOF
	}

	n, err := r.r.Read(p)
	r.n -= int64(n)
	return n, err
}

func (r *limitedReader) Close() error {
	return r.r.Close()
}
