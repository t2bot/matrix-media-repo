package util

import (
	"bytes"
	"compress/gzip"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"io"
)

func CompressBytesIfNeeded(b []byte, ctx rcontext.RequestContext) ([]byte, bool, error) {
	if !ctx.Config.Uploads.Compression.Enabled {
		return b, false, nil
	}

	buf := &bytes.Buffer{}
	w, err := gzip.NewWriterLevel(buf, ctx.Config.Uploads.Compression.Level)
	if err != nil {
		return nil, false, err
	}
	defer w.Close()

	_, err = w.Write(b)
	if err != nil {
		return nil, false, err
	}

	// Everything is written: close it out
	w.Close()

	return buf.Bytes(), true, nil
}

func DecompressBytesIfNeeded(s io.ReadCloser, compressed bool, ctx rcontext.RequestContext) (io.ReadCloser, error) {
	if !compressed {
		return s, nil
	}

	r, err := gzip.NewReader(s)
	if err != nil {
		return nil, err
	}
	return r, nil
}
