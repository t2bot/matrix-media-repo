package util

import (
	"io"
	"net/http"
	"strings"

	"github.com/h2non/filetype"
)

func GetMimeType(stream io.ReadCloser) (string, error) {
	defer DumpAndCloseStream(stream)

	// We only need the first 512 bytes at most to determine the file type
	buf := make([]byte, 512)
	_, err := stream.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}

	// Try identifying through the filetype repo first
	kind, err := filetype.Match(buf)
	if err != nil || kind == filetype.Unknown {
		// It's unknown or had a problem reading - try against the http lib
		contentType := http.DetectContentType(buf)
		contentType = strings.Split(contentType, ";")[0]

		// This shouldn't happen, but we'll check anyways. The http lib should return application/octet-stream
		// if it can't figure it out.
		if contentType == "" {
			contentType = "application/x-binary"
		}
		return contentType, nil
	}

	return kind.MIME.Value, nil
}
