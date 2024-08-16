package config

type GeneralConfig struct {
	BindAddress                string `yaml:"bindAddress"`
	Port                       int    `yaml:"port"`
	LogDirectory               string `yaml:"logDirectory"`
	LogColors                  bool   `yaml:"logColors"`
	JsonLogs                   bool   `yaml:"jsonLogs"`
	LogLevel                   string `yaml:"logLevel"`
	TrustAnyForward            bool   `yaml:"trustAnyForwardedAddress"`
	UseForwardedHost           bool   `yaml:"useForwardedHost"`
	FreezeUnauthenticatedMedia bool   `yaml:"freezeUnauthenticatedMedia"`
}

type HomeserverConfig struct {
	Name            string `yaml:"name"`
	ClientServerApi string `yaml:"csApi"`
	BackoffAt       int    `yaml:"backoffAt"`
	AdminApiKind    string `yaml:"adminApiKind"`
	SigningKeyPath  string `yaml:"signingKeyPath"`
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
	NumWorkers      int `yaml:"numWorkers"`
	ExpireDays      int `yaml:"expireAfterDays"`
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
	RequestsPerSecond float64                `yaml:"requestsPerSecond"`
	Enabled           bool                   `yaml:"enabled"`
	BurstCount        int                    `yaml:"burst"`
	Buckets           RateLimitBucketsConfig `yaml:"buckets"`
}

type RateLimitBucketsConfig struct {
	Downloads RateLimitDownloadBucketConfig `yaml:"downloads"`
}

type RateLimitDownloadBucketConfig struct {
	CapacityBytes       int64 `yaml:"capacityBytes"`
	DrainBytesPerMinute int64 `yaml:"drainBytesPerMinute"`
	OverflowLimitBytes  int64 `yaml:"overflowLimitBytes"`
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
	BackoffAt    int      `yaml:"backoffAt"`
	IgnoredHosts []string `yaml:"ignoredHosts,flow"`
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
	DbNum   int                `default:"0" yaml:"databaseNumber"`
}

type RedisShardConfig struct {
	Name    string `yaml:"name"`
	Address string `yaml:"addr"`
}

type TasksConfig struct {
	NumWorkers int `yaml:"numWorkers"`
}

type PGOConfig struct {
	Enabled   bool   `yaml:"enabled"`
	SubmitUrl string `yaml:"submitUrl"`
	SubmitKey string `yaml:"submitKey"`
}
