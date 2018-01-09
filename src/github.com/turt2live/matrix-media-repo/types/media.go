package types

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

func (m *Media) MxcUri() string {
	return "mxc://" + m.Origin + "/" + m.MediaId
}
