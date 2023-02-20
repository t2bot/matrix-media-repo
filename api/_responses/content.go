package _responses

import "io"

type EmptyResponse struct{}

type HtmlResponse struct {
	HTML string
}

type DownloadResponse struct {
	ContentType       string
	Filename          string
	SizeBytes         int64
	Data              io.ReadCloser
	TargetDisposition string
}

type StreamDataResponse struct {
	Stream io.Reader
}
