package stream_util

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/turt2live/matrix-media-repo/util/util_byte_seeker"
)

func BufferToStream(buf *bytes.Buffer) io.ReadCloser {
	newBuf := bytes.NewReader(buf.Bytes())
	return io.NopCloser(newBuf)
}

func BytesToStream(b []byte) io.ReadCloser {
	return io.NopCloser(bytes.NewBuffer(b))
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
	defer DumpAndCloseStream(r)

	hasher := sha256.New()

	if _, err := io.Copy(hasher, r); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func ClonedBufReader(buf bytes.Buffer) util_byte_seeker.ByteSeeker {
	return util_byte_seeker.NewByteSeeker(buf.Bytes())
}

func ForceDiscard(r io.Reader, nBytes int64) error {
	if nBytes == 0 {
		return nil // weird call, but ok
	}

	buf := make([]byte, 128)

	if nBytes < 0 {
		for true {
			_, err := r.Read(buf)
			if err == io.EOF {
				break
			} else if err != nil {
				return err
			}
		}
		return nil
	}

	read := int64(0)
	for (nBytes - read) > 0 {
		toRead := int(math.Min(float64(len(buf)), float64(nBytes-read)))
		if toRead != len(buf) {
			buf = make([]byte, toRead)
		}
		actuallyRead, err := r.Read(buf)
		if err != nil {
			return err
		}
		read += int64(actuallyRead)
		if (nBytes - read) < 0 {
			return errors.New(fmt.Sprintf("over-discarded from stream by %d bytes", nBytes-read))
		}
	}

	return nil
}

func ManualSeekStream(r io.Reader, bytesStart int64, bytesToRead int64) (io.Reader, error) {
	if sr, ok := r.(io.ReadSeeker); ok {
		_, err := sr.Seek(bytesStart, io.SeekStart)
		if err != nil {
			return nil, err
		}
	} else {
		err := ForceDiscard(r, bytesStart)
		if err != nil {
			return nil, err
		}
	}
	return io.LimitReader(r, bytesToRead), nil
}

func DumpAndCloseStream(r io.ReadCloser) {
	if r == nil {
		return // nothing to dump or close
	}
	_ = ForceDiscard(r, -1)
	_ = r.Close()
}
