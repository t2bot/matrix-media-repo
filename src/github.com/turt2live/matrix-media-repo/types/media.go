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

type MinimalMedia struct {
	Origin      string
	MediaId     string
	Stream      io.ReadCloser
	UploadName  string
	ContentType string
	SizeBytes   int64
	KnownMedia  *Media
}

func (m *Media) MxcUri() string {
	return "mxc://" + m.Origin + "/" + m.MediaId
}
