package cleanup

import (
	"io"
	"io/ioutil"
)

func DumpAndCloseStream(r io.ReadCloser) {
	if r == nil {
		return // nothing to dump or close
	}
	_, _ = io.Copy(ioutil.Discard, r)
	_ = r.Close()
}
