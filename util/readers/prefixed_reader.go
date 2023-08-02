package readers

import (
	"bytes"
	"io"
)

type PrefixedReader struct {
	io.Reader
	b        *bytes.Buffer
	r        io.Reader
	inBuffer bool
}

func NewPrefixedReader(prefix *bytes.Buffer, r io.Reader) *PrefixedReader {
	return &PrefixedReader{
		b:        prefix,
		r:        r,
		inBuffer: prefix.Len() > 0,
	}
}

func (r *PrefixedReader) Read(p []byte) (int, error) {
	if r.inBuffer {
		read, err := r.b.Read(p)
		if err == io.EOF {
			r.inBuffer = false
			return read, nil
		}
		if r.b.Len() <= 0 {
			r.inBuffer = false
		}
		return read, err
	}
	return r.r.Read(p)
}
