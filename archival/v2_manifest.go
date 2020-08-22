package archival

type V2ManifestRecord struct {
	FileName     string `json:"name"`
	ArchivedName string `json:"file_name"`
	SizeBytes    int64  `json:"size_bytes"`
	ContentType  string `json:"content_type"`
	S3Url        string `json:"s3_url"`
	Sha256       string `json:"sha256"`
	Origin       string `json:"origin"`
	MediaId      string `json:"media_id"`
	CreatedTs    int64  `json:"created_ts"`
	Uploader     string `json:"uploader"`
}

type V2Manifest struct {
	Version   int                          `json:"version"`
	EntityId  string                       `json:"entity_id"`
	CreatedTs int64                        `json:"created_ts"`
	Media     map[string]*V2ManifestRecord `json:"media"`
}
