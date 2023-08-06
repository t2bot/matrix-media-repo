package readers

import "io"

type RewindReader struct {
	io.ReadCloser
	r io.ReadSeeker
}

func NewRewindReader(r io.ReadSeeker) *RewindReader {
	return &RewindReader{
		r: r,
	}
}

func (r *RewindReader) Read(p []byte) (int, error) {
	return r.r.Read(p)
}

func (r *RewindReader) Close() error {
	_, err := r.r.Seek(0, io.SeekStart)
	return err
}
