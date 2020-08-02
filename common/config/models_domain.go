package config

type ArchivingConfig struct {
	Enabled            bool  `yaml:"enabled"`
	SelfService        bool  `yaml:"selfService"`
	TargetBytesPerPart int64 `yaml:"targetBytesPerPart"`
}

type QuotaUserConfig struct {
	Glob     string `yaml:"glob"`
	MaxBytes int64  `yaml:"maxBytes"`
}

type QuotasConfig struct {
	Enabled    bool              `yaml:"enabled"`
	UserQuotas []QuotaUserConfig `yaml:"users,flow"`
}

type UploadsConfig struct {
	MaxSizeBytes         int64        `yaml:"maxBytes"`
	MinSizeBytes         int64        `yaml:"minBytes"`
	ReportedMaxSizeBytes int64        `yaml:"reportedMaxBytes"`
	Quota                QuotasConfig `yaml:"quotas"`
}

type DatastoreConfig struct {
	Type       string            `yaml:"type"`
	Enabled    bool              `yaml:"enabled"`
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
	DynamicSizing       bool            `yaml:"dynamicSizing"`
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
	DefaultLanguage    string   `yaml:"defaultLanguage"`
	OEmbed             bool     `yaml:"oEmbed"`
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

type FeatureConfig struct {
	MSC2448Blurhash MSC2448Config `yaml:"MSC2448"`
	IPFS            IPFSConfig    `yaml:"IPFS"`
	Redis           RedisConfig   `yaml:"redis"`
}

type MSC2448Config struct {
	Enabled         bool `yaml:"enabled"`
	MaxRenderWidth  int  `yaml:"maxWidth"`
	MaxRenderHeight int  `yaml:"maxHeight"`
	GenerateWidth   int  `yaml:"thumbWidth"`
	GenerateHeight  int  `yaml:"thumbHeight"`
	XComponents     int  `yaml:"xComponents"`
	YComponents     int  `yaml:"yComponents"`
	Punch           int  `yaml:"punch"`
}

type IPFSConfig struct {
	Enabled bool             `yaml:"enabled"`
	Daemon  IPFSDaemonConfig `yaml:"builtInDaemon"`
}

type IPFSDaemonConfig struct {
	Enabled  bool   `yaml:"enabled"`
	RepoPath string `yaml:"repoPath"`
}

type RedisConfig struct {
	Enabled bool               `yaml:"enabled"`
	Shards  []RedisShardConfig `yaml:"shards,flow"`
}

type RedisShardConfig struct {
	Name    string `yaml:"name"`
	Address string `yaml:"addr"`
}

type AccessTokenConfig struct {
	MaxCacheTimeSeconds int                `yaml:"maxCacheTimeSeconds"`
	UseAppservices      bool               `yaml:"useLocalAppserviceConfig"`
	Appservices         []AppserviceConfig `yaml:"appservices,flow"`
}

type AppserviceConfig struct {
	Id              string                          `yaml:"id"`
	AppserviceToken string                          `yaml:"asToken"`
	SenderUserId    string                          `yaml:"senderUserId"`
	UserNamespaces  []AppserviceUserNamespaceConfig `yaml:"userNamespaces,flow"`
}

type AppserviceUserNamespaceConfig struct {
	Regex string `yaml:"regex"`
}
