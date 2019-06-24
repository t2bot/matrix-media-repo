package util

import (
	"bytes"
	"io"
	"io/ioutil"

	"github.com/sirupsen/logrus"
)

type ManyReader struct {
	buf *bytes.Buffer
	eof bool
}

type ManyReaderReader struct {
	manyReader *ManyReader
	pos int
}

func NewManyReader(input io.Reader) *ManyReader {
	var buf bytes.Buffer
	tr := io.TeeReader(input, &buf)

	mr := &ManyReader{&buf, false}
	go func() {
		ioutil.ReadAll(tr)
		mr.eof = true
	}()

	return mr
}

func (r *ManyReader) GetReader() *ManyReaderReader {
	return &ManyReaderReader{r, 0}
}

func (r *ManyReaderReader) Read(p []byte) (int, error) {
	b := r.manyReader.buf.Bytes()
	available := len(b)
	if r.pos >= available - 1 && r.manyReader.eof {
		return 0, io.EOF
	}

	limit := len(p)
	end := r.pos + limit
	if end > available {
		end = available
	}

	if end == r.pos || end <= 0 {
		return 0, nil
	}

	limit = end - r.pos
	if limit <= 0 {
		return 0, nil
	}

	logrus.Info("Available: ", available)
	logrus.Info("Position: ", r.pos)
	logrus.Info("End: ", end)
	logrus.Info("Limit: ", limit)

	for i := 0; i < limit; i++ {
		p[i] = b[r.pos + i]
	}
	r.pos += limit

	logrus.Info("Read: ", limit)
	logrus.Info("Final Position: ", r.pos)

	return limit, nil
}
