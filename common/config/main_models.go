package config

type GeneralConfig struct {
	BindAddress      string `yaml:"bindAddress"`
	Port             int    `yaml:"port"`
	LogDirectory     string `yaml:"logDirectory"`
	TrustAnyForward  bool   `yaml:"trustAnyForwardedAddress"`
	UseForwardedHost bool   `yaml:"useForwardedHost"`
}

type HomeserverConfig struct {
	Name            string `yaml:"name"`
	ClientServerApi string `yaml:"csApi"`
	BackoffAt       int    `yaml:"backoffAt"`
	AdminApiKind    string `yaml:"adminApiKind"`
}

type DatabaseConfig struct {
	Postgres string        `yaml:"postgres"`
	Pool     *DbPoolConfig `yaml:"pool"`
}

type DbPoolConfig struct {
	MaxConnections int `yaml:"maxConnections"`
	MaxIdle        int `yaml:"maxIdleConnections"`
}

type MainDownloadsConfig struct {
	DownloadsConfig `yaml:",inline"`
	NumWorkers      int         `yaml:"numWorkers"`
	Cache           CacheConfig `yaml:"cache"`
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

type MainThumbnailsConfig struct {
	ThumbnailsConfig `yaml:",inline"`
	NumWorkers       int `yaml:"numWorkers"`
}

type MainUrlPreviewsConfig struct {
	UrlPreviewsConfig `yaml:",inline"`
	NumWorkers        int `yaml:"numWorkers"`
}

type RateLimitConfig struct {
	RequestsPerSecond float64 `yaml:"requestsPerSecond"`
	Enabled           bool    `yaml:"enabled"`
	BurstCount        int     `yaml:"burst"`
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
