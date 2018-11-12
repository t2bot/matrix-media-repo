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
	DatastoreId string
	Location    string
	CreationTs  int64
	Quarantined bool
}

type StreamedMedia struct {
	Media  *Media
	Stream io.ReadCloser
}

func (m *Media) MxcUri() string {
	return "mxc://" + m.Origin + "/" + m.MediaId
}
