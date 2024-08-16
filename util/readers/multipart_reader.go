package readers

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/textproto"
	"net/url"

	"github.com/alioygur/is"
)

type MultipartPart struct {
	ContentType string
	FileName    string
	Location    string
	Reader      io.ReadCloser
}

func NewMultipartReader(boundary string, parts ...*MultipartPart) io.ReadCloser {
	r, w := io.Pipe()
	go func() {
		mpw := multipart.NewWriter(w)
		err := mpw.SetBoundary(boundary)
		if err != nil {
			// We don't have a good error route, and don't expect this to fail anyways.
			panic(err)
		}

		for _, part := range parts {
			headers := textproto.MIMEHeader{}
			if part.ContentType != "" {
				headers.Set("Content-Type", part.ContentType)
			}
			if part.FileName != "" {
				if is.ASCII(part.FileName) {
					headers.Set("Content-Disposition", "attachment; filename="+url.QueryEscape(part.FileName))
				} else {
					headers.Set("Content-Disposition", "attachment; filename*=utf-8''"+url.QueryEscape(part.FileName))
				}
			}
			if part.Location != "" {
				headers.Set("Location", part.Location)
				part.Reader = io.NopCloser(bytes.NewReader(make([]byte, 0)))
			}

			partW, err := mpw.CreatePart(headers)
			if err != nil {
				_ = w.CloseWithError(err)
				return
			}
			if _, err = io.Copy(partW, part.Reader); err != nil {
				_ = w.CloseWithError(err)
				return
			}
			if err = part.Reader.Close(); err != nil {
				_ = w.CloseWithError(err)
				return
			}
		}

		if err := mpw.Close(); err != nil {
			_ = w.CloseWithError(err)
		}
		_ = w.Close()
	}()
	return MakeCloser(r)
}
