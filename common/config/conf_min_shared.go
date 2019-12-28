package config

type MinimumRepoConfig struct {
	DataStores     []DatastoreConfig `yaml:"datastores"`
	Archiving      ArchivingConfig   `yaml:"archiving"`
	Uploads        UploadsConfig     `yaml:"uploads"`
	Identicons     IdenticonsConfig  `yaml:"identicons"`
	Quarantine     QuarantineConfig  `yaml:"quarantine"`
	TimeoutSeconds TimeoutsConfig    `yaml:"timeouts"`
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
			StoragePaths:         []string{},
			AllowedTypes:         []string{"*/*"},
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
	}
}
