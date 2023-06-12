package synapse

type LocalMedia struct {
	MediaId     string
	ContentType string
	SizeBytes   int64
	CreatedTs   int64
	UploadName  string
	UserId      string
	UrlCache    string
}
