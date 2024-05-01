package util

import (
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
)

type MatrixMediaPart struct {
	Header textproto.MIMEHeader
	Body   io.ReadCloser
}

func MatrixMediaPartFromResponse(r *http.Response) *MatrixMediaPart {
	return &MatrixMediaPart{
		Header: textproto.MIMEHeader(r.Header),
		Body:   r.Body,
	}
}

func MatrixMediaPartFromMimeMultipart(p *multipart.Part) *MatrixMediaPart {
	return &MatrixMediaPart{
		Header: p.Header,
		Body:   p,
	}
}
