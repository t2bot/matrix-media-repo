package types

type Datastore struct {
	DatastoreId string
	Type        string
	Uri         string
}

type DatastoreMigrationEstimate struct {
	ThumbnailsAffected      int64 `json:"thumbnails_affected"`
	ThumbnailHashesAffected int64 `json:"thumbnail_hashes_affected"`
	ThumbnailBytes          int64 `json:"thumbnail_bytes"`
	MediaAffected           int64 `json:"media_affected"`
	MediaHashesAffected     int64 `json:"media_hashes_affected"`
	MediaBytes              int64 `json:"media_bytes"`
	TotalHashesAffected     int64 `json:"total_hashes_affected"`
	TotalBytes              int64 `json:"total_bytes"`
}
