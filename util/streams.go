package util

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"

	"github.com/turt2live/matrix-media-repo/util/cleanup"
	"github.com/turt2live/matrix-media-repo/util/util_byte_seeker"
)

func BufferToStream(buf *bytes.Buffer) io.ReadCloser {
	newBuf := bytes.NewReader(buf.Bytes())
	return ioutil.NopCloser(newBuf)
}

func CloneReader(input io.ReadCloser, numReaders int) []io.ReadCloser {
	readers := make([]io.ReadCloser, 0)
	writers := make([]io.WriteCloser, 0)

	for i := 0; i < numReaders; i++ {
		r, w := io.Pipe()
		readers = append(readers, r)
		writers = append(writers, w)
	}

	go func() {
		plainWriters := make([]io.Writer, 0)
		for _, w := range writers {
			defer w.Close()
			plainWriters = append(plainWriters, w)
		}

		mw := io.MultiWriter(plainWriters...)
		io.Copy(mw, input)
	}()

	return readers
}

func GetSha256HashOfStream(r io.ReadCloser) (string, error) {
	defer cleanup.DumpAndCloseStream(r)

	hasher := sha256.New()

	if _, err := io.Copy(hasher, r); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func ClonedBufReader(buf bytes.Buffer) util_byte_seeker.ByteSeeker {
	return util_byte_seeker.NewByteSeeker(buf.Bytes())
}