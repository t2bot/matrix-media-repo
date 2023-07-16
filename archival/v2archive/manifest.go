package v2archive

const (
	ManifestVersionV1 = 1
	ManifestVersionV2 = 2
)
const ManifestVersion = ManifestVersionV2

type Manifest struct {
	Version   int                        `json:"version"`
	EntityId  string                     `json:"entity_id"`
	CreatedTs int64                      `json:"created_ts"`
	Media     map[string]*ManifestRecord `json:"media"`

	// Deprecated: for v1 manifests, now called EntityId
	UserId string `json:"user_id,omitempty"`
}

type ManifestRecord struct {
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
