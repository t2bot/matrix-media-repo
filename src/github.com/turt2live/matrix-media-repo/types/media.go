package types

import "io"

type Media struct {
	Origin      string
	MediaId     string
	UploadName  string
	ContentType string
	UserId      string
	Sha256Hash  string
	SizeBytes   int64
	Location    string
	CreationTs  int64
}

type StreamedMedia struct {
	Media  *Media
	Thumbnail *Thumbnail // Only set if the media represents a thumbnail
	Stream io.ReadCloser
}

func (m *Media) MxcUri() string {
	return "mxc://" + m.Origin + "/" + m.MediaId
}
