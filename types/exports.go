package types

type ExportMetadata struct {
	ExportID string
	Entity   string
}

type ExportPart struct {
	ExportID    string
	Index       int
	FileName    string
	SizeBytes   int64
	DatastoreID string
	Location    string
}
