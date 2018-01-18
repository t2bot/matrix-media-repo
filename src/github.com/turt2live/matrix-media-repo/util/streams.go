package util

import (
	"bytes"
	"io"
	"io/ioutil"
)

func GetStreamFromBuffer(buf *bytes.Buffer) io.ReadCloser {
	newBuf := bytes.NewReader(buf.Bytes())
	return ioutil.NopCloser(newBuf)
}
