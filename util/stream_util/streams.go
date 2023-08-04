package stream_util

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
)

func BufferToStream(buf *bytes.Buffer) io.ReadCloser {
	newBuf := bytes.NewReader(buf.Bytes())
	return io.NopCloser(newBuf)
}

func BytesToStream(b []byte) io.ReadCloser {
	return io.NopCloser(bytes.NewBuffer(b))
}

func GetSha256HashOfStream(r io.ReadCloser) (string, error) {
	defer DumpAndCloseStream(r)

	hasher := sha256.New()

	if _, err := io.Copy(hasher, r); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
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

func DumpAndCloseStream(r io.ReadCloser) {
	if r == nil {
		return // nothing to dump or close
	}
	_ = ForceDiscard(r, -1)
	_ = r.Close()
}
