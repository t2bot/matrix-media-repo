package config

type GeneralConfig struct {
	BindAddress      string `yaml:"bindAddress"`
	Port             int    `yaml:"port"`
	LogDirectory     string `yaml:"logDirectory"`
	LogColors        bool   `yaml:"logColors"`
	JsonLogs         bool   `yaml:"jsonLogs"`
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
	ExpireDays      int         `yaml:"expireAfterDays"`
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
	ExpireDays       int `yaml:"expireAfterDays"`
}

type MainUrlPreviewsConfig struct {
	UrlPreviewsConfig `yaml:",inline"`
	NumWorkers        int `yaml:"numWorkers"`
	ExpireDays        int `yaml:"expireAfterDays"`
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

type FederationConfig struct {
	BackoffAt int `yaml:"backoffAt"`
}

type PluginConfig struct {
	Executable string                 `yaml:"exec"`
	Config     map[string]interface{} `yaml:"config"`
}

type SentryConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Dsn         string `yaml:"dsn"`
	Environment string `yaml:"environment"`
	Debug       bool   `yaml:"debug"`
}

type RedisConfig struct {
	Enabled bool               `yaml:"enabled"`
	Shards  []RedisShardConfig `yaml:"shards,flow"`
}

type RedisShardConfig struct {
	Name    string `yaml:"name"`
	Address string `yaml:"addr"`
}