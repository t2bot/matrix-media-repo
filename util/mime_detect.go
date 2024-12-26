package util

import (
	"io"
	"net/http"
	"strings"

	"github.com/h2non/filetype"
)

func DetectMimeType(r io.ReadSeeker) (string, error) {
	buf := make([]byte, 512)

	current, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return "", err
	}
	restore := func() error {
		if _, err2 := r.Seek(current, io.SeekStart); err2 != nil {
			return err2
		}
		return nil
	}

	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	if _, err := r.Read(buf); err != nil {
		return "", err
	}

	kind, err := filetype.Match(buf)
	if err != nil || kind == filetype.Unknown {
		// Try against http library upon error
		contentType := http.DetectContentType(buf)
		contentType = strings.Split(contentType, ";")[0]

		// http should return an octet-stream anyway, but just in case:
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		return contentType, restore()
	}

	return kind.MIME.Value, restore()
}
