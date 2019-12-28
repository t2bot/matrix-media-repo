package config

type ArchivingConfig struct {
	Enabled            bool  `yaml:"enabled"`
	SelfService        bool  `yaml:"selfService"`
	TargetBytesPerPart int64 `yaml:"targetBytesPerPart"`
}

type UploadsConfig struct {
	StoragePaths         []string            `yaml:"storagePaths,flow"` // deprecated
	MaxSizeBytes         int64               `yaml:"maxBytes"`
	MinSizeBytes         int64               `yaml:"minBytes"`
	AllowedTypes         []string            `yaml:"allowedTypes,flow"`
	PerUserExclusions    map[string][]string `yaml:"exclusions,flow"`
	ReportedMaxSizeBytes int64               `yaml:"reportedMaxBytes"`
}

type DatastoreConfig struct {
	Type       string            `yaml:"type"`
	Enabled    bool              `yaml:"enabled"`
	ForUploads bool              `yaml:"forUploads"` // deprecated
	MediaKinds []string          `yaml:"forKinds,flow"`
	Options    map[string]string `yaml:"opts,flow"`
}

type DownloadsConfig struct {
	MaxSizeBytes        int64 `yaml:"maxBytes"`
	FailureCacheMinutes int   `yaml:"failureCacheMinutes"`
}

type ThumbnailsConfig struct {
	MaxSourceBytes      int64           `yaml:"maxSourceBytes"`
	Types               []string        `yaml:"types,flow"`
	MaxAnimateSizeBytes int64           `yaml:"maxAnimateSizeBytes"`
	Sizes               []ThumbnailSize `yaml:"sizes,flow"`
	AllowAnimated       bool            `yaml:"allowAnimated"`
	DefaultAnimated     bool            `yaml:"defaultAnimated"`
	StillFrame          float32         `yaml:"stillFrame"`
}

type ThumbnailSize struct {
	Width  int `yaml:"width"`
	Height int `yaml:"height"`
}

type UrlPreviewsConfig struct {
	Enabled            bool     `yaml:"enabled"`
	NumWords           int      `yaml:"numWords"`
	NumTitleWords      int      `yaml:"numTitleWords"`
	MaxLength          int      `yaml:"maxLength"`
	MaxTitleLength     int      `yaml:"maxTitleLength"`
	MaxPageSizeBytes   int64    `yaml:"maxPageSizeBytes"`
	FilePreviewTypes   []string `yaml:"filePreviewTypes,flow"`
	DisallowedNetworks []string `yaml:"disallowedNetworks,flow"`
	AllowedNetworks    []string `yaml:"allowedNetworks,flow"`
	UnsafeCertificates bool     `yaml:"previewUnsafeCertificates"`
}

type IdenticonsConfig struct {
	Enabled bool `yaml:"enabled"`
}

type QuarantineConfig struct {
	ReplaceThumbnails bool   `yaml:"replaceThumbnails"`
	ReplaceDownloads  bool   `yaml:"replaceDownloads"`
	ThumbnailPath     string `yaml:"thumbnailPath"`
	AllowLocalAdmins  bool   `yaml:"allowLocalAdmins"`
}

type TimeoutsConfig struct {
	UrlPreviews  int `yaml:"urlPreviewTimeoutSeconds"`
	Federation   int `yaml:"federationTimeoutSeconds"`
	ClientServer int `yaml:"clientServerTimeoutSeconds"`
}
