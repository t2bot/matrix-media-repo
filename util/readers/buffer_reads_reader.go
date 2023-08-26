package readers

import (
	"bytes"
	"errors"
	"io"
)

type BufferReadsReader struct {
	io.Reader
	r        io.Reader
	original io.Reader
	b        *bytes.Buffer
	pr       io.Reader
}

func NewBufferReadsReader(r io.Reader) *BufferReadsReader {
	buf := bytes.NewBuffer(make([]byte, 0))
	tee := io.TeeReader(r, buf)
	return &BufferReadsReader{
		r:        tee,
		b:        buf,
		original: r,
		pr:       nil,
	}
}

func (r *BufferReadsReader) Read(p []byte) (int, error) {
	if r.pr != nil {
		return 0, errors.New("cannot read from this stream anymore - use the created prefixed reader")
	}
	return r.r.Read(p)
}

func (r *BufferReadsReader) MakeRewoundReader() (io.Reader, error) {
	if r.pr != nil {
		return r.pr, errors.New("prefixed reader already created from this reader")
	}
	r.pr = io.MultiReader(r.b, r.original)
	return r.pr, nil
}

func (r *BufferReadsReader) GetRewoundReader() io.Reader {
	pr, _ := r.MakeRewoundReader()
	return pr
}

func (r *BufferReadsReader) Discard() {
	r.b.Truncate(0)
}
