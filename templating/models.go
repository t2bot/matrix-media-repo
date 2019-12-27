package templating

type ViewExportPartModel struct {
	ExportID       string
	Index          int
	SizeBytes      int64
	SizeBytesHuman string
	FileName       string
}

type ViewExportModel struct {
	ExportID    string
	Entity      string
	ExportParts []*ViewExportPartModel
}

type ExportIndexMediaModel struct {
	ExportID        string
	ArchivedName    string
	FileName        string
	Origin          string
	MediaID         string
	SizeBytes       int64
	SizeBytesHuman  string
	UploadTs        int64
	UploadDateHuman string
	Sha256Hash      string
	ContentType     string
	Uploader        string
}

type ExportIndexModel struct {
	ExportID string
	Entity   string
	Media    []*ExportIndexMediaModel
}
