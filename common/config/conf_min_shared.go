package config

type MinimumRepoConfig struct {
	DataStores     []DatastoreConfig `yaml:"datastores"`
	Archiving      ArchivingConfig   `yaml:"archiving"`
	Uploads        UploadsConfig     `yaml:"uploads"`
	Identicons     IdenticonsConfig  `yaml:"identicons"`
	Quarantine     QuarantineConfig  `yaml:"quarantine"`
	TimeoutSeconds TimeoutsConfig    `yaml:"timeouts"`
	Features       FeatureConfig     `yaml:"featureSupport"`
	AccessTokens   AccessTokenConfig `yaml:"accessTokens"`
}

func NewDefaultMinimumRepoConfig() MinimumRepoConfig {
	return MinimumRepoConfig{
		DataStores: []DatastoreConfig{},
		Archiving: ArchivingConfig{
			Enabled:            true,
			SelfService:        false,
			TargetBytesPerPart: 209715200, // 200mb
		},
		Uploads: UploadsConfig{
			MaxSizeBytes:         104857600, // 100mb
			MinSizeBytes:         100,
			ReportedMaxSizeBytes: 0,
			Quota: QuotasConfig{
				Enabled:    false,
				UserQuotas: []QuotaUserConfig{},
			},
		},
		Identicons: IdenticonsConfig{
			Enabled: true,
		},
		Quarantine: QuarantineConfig{
			ReplaceThumbnails: true,
			ReplaceDownloads:  false,
			ThumbnailPath:     "",
			AllowLocalAdmins:  true,
		},
		TimeoutSeconds: TimeoutsConfig{
			UrlPreviews:  10,
			ClientServer: 30,
			Federation:   120,
		},
		Features: FeatureConfig{
			MSC2448Blurhash: MSC2448Config{
				Enabled:         false,
				MaxRenderWidth:  1024,
				MaxRenderHeight: 1024,
				GenerateWidth:   64,
				GenerateHeight:  64,
				XComponents:     4,
				YComponents:     3,
				Punch:           1,
			},
			IPFS: IPFSConfig{
				Enabled: false,
				Daemon: IPFSDaemonConfig{
					Enabled:  true,
					RepoPath: "./ipfs",
				},
			},
			Redis: RedisConfig{
				Enabled: false,
				Shards:  []RedisShardConfig{},
			},
		},
		AccessTokens: AccessTokenConfig{
			MaxCacheTimeSeconds: 0,
			UseAppservices:      false,
			Appservices:         []AppserviceConfig{},
		},
	}
}
