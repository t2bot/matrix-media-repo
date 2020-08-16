package util_byte_seeker

import (
	"bytes"
)

type ByteSeeker struct {
	s *bytes.Reader
}

func NewByteSeeker(b []byte) ByteSeeker {
	return ByteSeeker{s: bytes.NewReader(b)}
}

func (s ByteSeeker) Close() error {
	return nil // no-op
}

func (s ByteSeeker) Read(p []byte) (n int, err error) {
	return s.s.Read(p)
}

func (s ByteSeeker) Seek(offset int64, whence int) (int64, error) {
	return s.s.Seek(offset, whence)
}
