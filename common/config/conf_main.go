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
}

func NewDefaultMainConfig() MainRepoConfig {
	return MainRepoConfig{
		MinimumRepoConfig: NewDefaultMinimumRepoConfig(),
		General: GeneralConfig{
			BindAddress:      "127.0.0.1",
			Port:             8000,
			LogDirectory:     "logs",
			TrustAnyForward:  false,
			UseForwardedHost: true,
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
				MaxSizeBytes:        104857600, // 100mb
				FailureCacheMinutes: 15,
			},
			NumWorkers: 10,
			Cache: CacheConfig{
				Enabled:               true,
				MaxSizeBytes:          1048576000, // 1gb
				MaxFileSizeBytes:      104857600,  // 100mb
				TrackedMinutes:        30,
				MinDownloads:          5,
				MinCacheTimeSeconds:   300, // 5min
				MinEvictedTimeSeconds: 60,
			},
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
					"0.0.0.0/0", // "Everything"
				},
				DefaultLanguage: "en-US,en",
				OEmbed:          false,
			},
			NumWorkers: 10,
			ExpireDays: 0,
		},
		Thumbnails: MainThumbnailsConfig{
			ThumbnailsConfig: ThumbnailsConfig{
				MaxSourceBytes:      10485760, // 10mb
				MaxAnimateSizeBytes: 10485760, // 10mb
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
			BackoffAt: 20,
		},
	}
}
