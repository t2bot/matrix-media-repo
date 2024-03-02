package preview

import "io"

type readSeekerWrapper struct {
	data   []byte
	offset int64
}

func (r *readSeekerWrapper) Read(p []byte) (n int, err error) {
	if r.offset >= int64(len(r.data)) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.offset:])
	r.offset += int64(n)
	return
}

func (r *readSeekerWrapper) Seek(offset int64, whence int) (int64, error) {
	var absOffset int64
	switch whence {
	case io.SeekStart:
		absOffset = offset
	case io.SeekCurrent:
		absOffset = r.offset + offset
	case io.SeekEnd:
		absOffset = int64(len(r.data)) + offset
	}
	if absOffset < 0 || absOffset > int64(len(r.data)) {
		return 0, io.EOF
	}
	r.offset = absOffset
	return absOffset, nil
}

func newReadSeekerWrapper(reader io.Reader) (*readSeekerWrapper, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return &readSeekerWrapper{data: data}, nil
}
