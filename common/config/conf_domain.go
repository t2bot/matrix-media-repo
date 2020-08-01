package config

type DomainRepoConfig struct {
	MinimumRepoConfig `yaml:",inline"`
	HomeserverConfig  `yaml:",inline"`
	Downloads         DownloadsConfig   `yaml:"downloads"`
	Thumbnails        ThumbnailsConfig  `yaml:"thumbnails"`
	UrlPreviews       UrlPreviewsConfig `yaml:"urlPreviews"`
}

func NewDefaultDomainConfig() DomainRepoConfig {
	return DomainRepoConfig{
		MinimumRepoConfig: NewDefaultMinimumRepoConfig(),
		HomeserverConfig: HomeserverConfig{
			Name:            "UNDEFINED",
			ClientServerApi: "https://UNDEFINED",
			BackoffAt:       10,
			AdminApiKind:    "matrix",
		},
		Downloads: DownloadsConfig{
			MaxSizeBytes:        104857600, // 100mb
			FailureCacheMinutes: 15,
		},
		UrlPreviews: UrlPreviewsConfig{
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
		Thumbnails: ThumbnailsConfig{
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
	}
}
