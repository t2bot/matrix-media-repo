package types

import (
	"encoding/json"
	"io"
)

type MediaRef struct {
	Origin  string
	MediaId string
}

func (ref MediaRef) MarshalBinary() ([]byte, error) {
	return json.Marshal(ref)
}

func (ref *MediaRef) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, ref)
}

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

type MinimalMediaMetadata struct {
	SizeBytes    int64
	Sha256Hash   string
	Location     string
	CreationTs   int64
	LastAccessTs int64
	DatastoreId  string
}

func (m *Media) MxcUri() string {
	return "mxc://" + m.Origin + "/" + m.MediaId
}
