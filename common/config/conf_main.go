package config

type MainRepoConfig struct {
	MinimumRepoConfig `yaml:",inline"`
	General           GeneralConfig         `yaml:"repo"`
	Homeservers       []HomeserverConfig    `yaml:"homeservers,flow"`
	Admins            []string              `yaml:"admins,flow"`
	Database          DatabaseConfig        `yaml:"database"`
	Downloads         MainDownloadsConfig   `yaml:"downloads"`
	Thumbnails        MainThumbnailsConfig  `yaml:"thumbnails"`
	UrlPreviews       MainUrlPreviewsConfig `yaml:"urlPreviews"`
	RateLimit         RateLimitConfig       `yaml:"rateLimit"`
	Metrics           MetricsConfig         `yaml:"metrics"`
	SharedSecret      SharedSecretConfig    `yaml:"sharedSecretAuth"`
	Federation        FederationConfig      `yaml:"federation"`
	Plugins           []PluginConfig        `yaml:"plugins,flow"`
	Sentry            SentryConfig          `yaml:"sentry"`
	Redis             RedisConfig           `yaml:"redis"`
	Tasks             TasksConfig           `yaml:"tasks"`
	PGO               PGOConfig             `yaml:"pgo"`
}

func NewDefaultMainConfig() MainRepoConfig {
	return MainRepoConfig{
		MinimumRepoConfig: NewDefaultMinimumRepoConfig(),
		General: GeneralConfig{
			BindAddress:                "127.0.0.1",
			Port:                       8000,
			LogDirectory:               "logs",
			LogColors:                  false,
			JsonLogs:                   false,
			LogLevel:                   "info",
			TrustAnyForward:            false,
			UseForwardedHost:           true,
			FreezeUnauthenticatedMedia: true,
		},
		Database: DatabaseConfig{
			Postgres: "postgres://your_username:your_password@localhost/database_name?sslmode=disable",
			Pool: &DbPoolConfig{
				MaxConnections: 25,
				MaxIdle:        5,
			},
		},
		Homeservers: []HomeserverConfig{},
		Admins:      []string{},
		Downloads: MainDownloadsConfig{
			DownloadsConfig: DownloadsConfig{
				MaxSizeBytes:               104857600, // 100mb
				FailureCacheMinutes:        15,
				DefaultRangeChunkSizeBytes: 10485760, // 10mb
			},
			NumWorkers: 10,
			ExpireDays: 0,
		},
		UrlPreviews: MainUrlPreviewsConfig{
			UrlPreviewsConfig: UrlPreviewsConfig{
				Enabled:          true,
				NumWords:         50,
				NumTitleWords:    30,
				MaxLength:        200,
				MaxTitleLength:   150,
				MaxPageSizeBytes: 10485760, // 10mb
				FilePreviewTypes: []string{
					"image/*",
				},
				DisallowedNetworks: []string{
					// Keep in sync with federation.disallowedNetworks
					"127.0.0.1/8",
					"10.0.0.0/8",
					"172.16.0.0/12",
					"192.168.0.0/16",
					"100.64.0.0/10",
					"169.254.0.0/16",
					"::1/128",
					"fe80::/64",
					"fc00::/7",
				},
				AllowedNetworks: []string{
					// Keep in sync with federation.allowedNetworks
					"0.0.0.0/0", // "Everything"
				},
				DefaultLanguage: "en-US,en",
				UserAgent:       "matrix-media-repo",
				OEmbed:          false,
				ProxyURL:        "",
			},
			NumWorkers: 10,
			ExpireDays: 0,
		},
		Thumbnails: MainThumbnailsConfig{
			ThumbnailsConfig: ThumbnailsConfig{
				MaxSourceBytes:      10485760, // 10mb
				MaxAnimateSizeBytes: 10485760, // 10mb
				MaxPixels:           32000000, // 32M
				AllowAnimated:       true,
				DefaultAnimated:     false,
				StillFrame:          0.5,
				Sizes: []ThumbnailSize{
					{32, 32},
					{96, 96},
					{320, 240},
					{640, 480},
					{800, 600},
				},
				DynamicSizing: false,
				Types: []string{
					"image/jpeg",
					"image/jpg",
					"image/png",
					"image/gif",
				},
			},
			NumWorkers: 10,
			ExpireDays: 0,
		},
		RateLimit: RateLimitConfig{
			Enabled:           true,
			RequestsPerSecond: 5,
			BurstCount:        10,
			Buckets: RateLimitBucketsConfig{
				Downloads: RateLimitDownloadBucketConfig{
					CapacityBytes:       524288000, // 500mb
					DrainBytesPerMinute: 5242880,   // 5mb
					OverflowLimitBytes:  104857600, // 100mb
				},
			},
		},
		Metrics: MetricsConfig{
			Enabled:     false,
			BindAddress: "localhost",
			Port:        9000,
		},
		SharedSecret: SharedSecretConfig{
			Enabled: false,
			Token:   "ReplaceMe",
		},
		Federation: FederationConfig{
			BackoffAt:    20,
			IgnoredHosts: make([]string, 0),
			DisallowedNetworks: []string{
				// Keep in sync with urlPreviews.disallowedNetworks
				"127.0.0.1/8",
				"10.0.0.0/8",
				"172.16.0.0/12",
				"192.168.0.0/16",
				"100.64.0.0/10",
				"169.254.0.0/16",
				"::1/128",
				"fe80::/64",
				"fc00::/7",
			},
			AllowedNetworks: []string{
				// Keep in sync with urlPreviews.allowedNetworks
				"0.0.0.0/0", // "Everything"
			},
		},
		Plugins: []PluginConfig{},
		Sentry: SentryConfig{
			Enabled:     false,
			Dsn:         "not supplied",
			Environment: "",
			Debug:       false,
		},
		Redis: RedisConfig{
			Enabled: false,
			Shards:  []RedisShardConfig{},
		},
		Tasks: TasksConfig{
			NumWorkers: 5,
		},
		PGO: PGOConfig{
			Enabled:   false,
			SubmitUrl: "https://mmr-pgo.t2host.io/v1/submit",
			SubmitKey: "",
		},
	}
}
