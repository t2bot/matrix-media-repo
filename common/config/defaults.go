package config

func NewDefaultConfig() *MediaRepoConfig {
	return &MediaRepoConfig{
		General: &GeneralConfig{
			BindAddress:      "127.0.0.1",
			Port:             8000,
			LogDirectory:     "logs",
			TrustAnyForward:  false,
			UseForwardedHost: true,
		},
		Database: &DatabaseConfig{
			Postgres: "postgres://your_username:your_password@localhost/database_name?sslmode=disable",
			Pool: &DbPoolConfig{
				MaxConnections: 25,
				MaxIdle:        5,
			},
		},
		Homeservers: []*HomeserverConfig{},
		Admins:      []string{},
		DataStores:  []DatastoreConfig{},
		Archiving: &ArchivingConfig{
			Enabled:            true,
			SelfService:        false,
			TargetBytesPerPart: 209715200, // 200mb
		},
		Uploads: &UploadsConfig{
			MaxSizeBytes:         104857600, // 100mb
			MinSizeBytes:         100,
			ReportedMaxSizeBytes: 0,
			StoragePaths:         []string{},
			AllowedTypes:         []string{"*/*"},
		},
		Downloads: &DownloadsConfig{
			MaxSizeBytes:        104857600, // 100mb
			NumWorkers:          10,
			FailureCacheMinutes: 15,
			Cache: &CacheConfig{
				Enabled:               true,
				MaxSizeBytes:          1048576000, // 1gb
				MaxFileSizeBytes:      104857600,  // 100mb
				TrackedMinutes:        30,
				MinDownloads:          5,
				MinCacheTimeSeconds:   300, // 5min
				MinEvictedTimeSeconds: 60,
			},
		},
		UrlPreviews: &UrlPreviewsConfig{
			Enabled:          true,
			NumWords:         50,
			NumTitleWords:    30,
			MaxLength:        200,
			MaxTitleLength:   150,
			MaxPageSizeBytes: 10485760, // 10mb
			NumWorkers:       10,
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
		},
		Thumbnails: &ThumbnailsConfig{
			MaxSourceBytes:      10485760, // 10mb
			MaxAnimateSizeBytes: 10485760, // 10mb
			NumWorkers:          10,
			AllowAnimated:       true,
			DefaultAnimated:     false,
			StillFrame:          0.5,
			Sizes: []*ThumbnailSize{
				{32, 32},
				{96, 96},
				{320, 240},
				{640, 480},
				{800, 600},
			},
			Types: []string{
				"image/jpeg",
				"image/jpg",
				"image/png",
				"image/gif",
			},
		},
		RateLimit: &RateLimitConfig{
			Enabled:           true,
			RequestsPerSecond: 5,
			BurstCount:        10,
		},
		Identicons: &IdenticonsConfig{
			Enabled: true,
		},
		Quarantine: &QuarantineConfig{
			ReplaceThumbnails: true,
			ReplaceDownloads:  false,
			ThumbnailPath:     "",
			AllowLocalAdmins:  true,
		},
		TimeoutSeconds: &TimeoutsConfig{
			UrlPreviews:  10,
			ClientServer: 30,
			Federation:   120,
		},
		Metrics: &MetricsConfig{
			Enabled:     false,
			BindAddress: "localhost",
			Port:        9000,
		},
		SharedSecret: &SharedSecretConfig{
			Enabled: false,
			Token:   "ReplaceMe",
		},
	}
}
