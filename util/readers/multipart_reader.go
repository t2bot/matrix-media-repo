package readers

import (
	"io"
	"mime/multipart"
	"net/textproto"
	"net/url"

	"github.com/alioygur/is"
)

type MultipartPart struct {
	ContentType string
	FileName    string
	Reader      io.ReadCloser
}

func NewMultipartReader(parts ...*MultipartPart) io.ReadCloser {
	r, w := io.Pipe()
	go func() {
		mpw := multipart.NewWriter(w)

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
