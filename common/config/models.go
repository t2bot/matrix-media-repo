package config

type HomeserverConfig struct {
	Name            string `yaml:"name"`
	ClientServerApi string `yaml:"csApi"`
	BackoffAt       int    `yaml:"backoffAt"`
	AdminApiKind    string `yaml:"adminApiKind"`
}

type GeneralConfig struct {
	BindAddress      string `yaml:"bindAddress"`
	Port             int    `yaml:"port"`
	LogDirectory     string `yaml:"logDirectory"`
	TrustAnyForward  bool   `yaml:"trustAnyForwardedAddress"`
	UseForwardedHost bool   `yaml:"useForwardedHost"`
}

type DbPoolConfig struct {
	MaxConnections int `yaml:"maxConnections"`
	MaxIdle        int `yaml:"maxIdleConnections"`
}

type DatabaseConfig struct {
	Postgres string        `yaml:"postgres"`
	Pool     *DbPoolConfig `yaml:"pool"`
}

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
	MaxSizeBytes        int64        `yaml:"maxBytes"`
	NumWorkers          int          `yaml:"numWorkers"`
	FailureCacheMinutes int          `yaml:"failureCacheMinutes"`
	Cache               *CacheConfig `yaml:"cache"`
}

type ThumbnailsConfig struct {
	MaxSourceBytes      int64            `yaml:"maxSourceBytes"`
	NumWorkers          int              `yaml:"numWorkers"`
	Types               []string         `yaml:"types,flow"`
	MaxAnimateSizeBytes int64            `yaml:"maxAnimateSizeBytes"`
	Sizes               []*ThumbnailSize `yaml:"sizes,flow"`
	AllowAnimated       bool             `yaml:"allowAnimated"`
	DefaultAnimated     bool             `yaml:"defaultAnimated"`
	StillFrame          float32          `yaml:"stillFrame"`
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
	NumWorkers         int      `yaml:"numWorkers"`
	FilePreviewTypes   []string `yaml:"filePreviewTypes,flow"`
	DisallowedNetworks []string `yaml:"disallowedNetworks,flow"`
	AllowedNetworks    []string `yaml:"allowedNetworks,flow"`
	UnsafeCertificates bool     `yaml:"previewUnsafeCertificates"`
}

type RateLimitConfig struct {
	RequestsPerSecond float64 `yaml:"requestsPerSecond"`
	Enabled           bool    `yaml:"enabled"`
	BurstCount        int     `yaml:"burst"`
}

type IdenticonsConfig struct {
	Enabled bool `yaml:"enabled"`
}

type CacheConfig struct {
	Enabled               bool  `yaml:"enabled"`
	MaxSizeBytes          int64 `yaml:"maxSizeBytes"`
	MaxFileSizeBytes      int64 `yaml:"maxFileSizeBytes"`
	TrackedMinutes        int   `yaml:"trackedMinutes"`
	MinCacheTimeSeconds   int   `yaml:"minCacheTimeSeconds"`
	MinEvictedTimeSeconds int   `yaml:"minEvictedTimeSeconds"`
	MinDownloads          int   `yaml:"minDownloads"`
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

type MetricsConfig struct {
	Enabled     bool   `yaml:"enabled"`
	BindAddress string `yaml:"bindAddress"`
	Port        int    `yaml:"port"`
}

type SharedSecretConfig struct {
	Enabled bool   `yaml:"enabled"`
	Token   string `yaml:"token"`
}

type MediaRepoConfig struct {
	General        *GeneralConfig      `yaml:"repo"`
	Homeservers    []*HomeserverConfig `yaml:"homeservers,flow"`
	Admins         []string            `yaml:"admins,flow"`
	Database       *DatabaseConfig     `yaml:"database"`
	DataStores     []DatastoreConfig   `yaml:"datastores"`
	Archiving      *ArchivingConfig    `yaml:"archiving"`
	Uploads        *UploadsConfig      `yaml:"uploads"`
	Downloads      *DownloadsConfig    `yaml:"downloads"`
	Thumbnails     *ThumbnailsConfig   `yaml:"thumbnails"`
	UrlPreviews    *UrlPreviewsConfig  `yaml:"urlPreviews"`
	RateLimit      *RateLimitConfig    `yaml:"rateLimit"`
	Identicons     *IdenticonsConfig   `yaml:"identicons"`
	Quarantine     *QuarantineConfig   `yaml:"quarantine"`
	TimeoutSeconds *TimeoutsConfig     `yaml:"timeouts"`
	Metrics        *MetricsConfig      `yaml:"metrics"`
	SharedSecret   *SharedSecretConfig `yaml:"sharedSecretAuth"`
}
