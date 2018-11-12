package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"gopkg.in/yaml.v2"
)

type runtimeConfig struct {
	MigrationsPath string
}

var Runtime = &runtimeConfig{}

type HomeserverConfig struct {
	Name            string `yaml:"name"`
	ClientServerApi string `yaml:"csApi"`
	BackoffAt       int    `yaml:"backoffAt"`
}

type GeneralConfig struct {
	BindAddress  string `yaml:"bindAddress"`
	Port         int    `yaml:"port"`
	LogDirectory string `yaml:"logDirectory"`
}

type DatabaseConfig struct {
	Postgres string `yaml:"postgres"`
}

type UploadsConfig struct {
	StoragePaths         []string            `yaml:"storagePaths,flow"`
	MaxSizeBytes         int64               `yaml:"maxBytes"`
	AllowedTypes         []string            `yaml:"allowedTypes,flow"`
	AllowedExcl          map[string][]string `yaml:"exclusions,flow"`
	ReportedMaxSizeBytes int64               `yaml:"reportedMaxBytes"`
}

type DownloadsConfig struct {
	MaxSizeBytes        int64        `yaml:"maxBytes"`
	NumWorkers          int          `yaml:"numWorkers"`
	FailureCacheMinutes int          `yaml:"failureCacheMinutes"`
	Cache               *CacheConfig `yaml:"cache"`
}

type ThumbnailsConfig struct {
	MaxSourceBytes      int64            `yaml:"maxSourceBytes"`
	NumWorkers          int              `yaml:"numWorkers"`
	Types               []string         `yaml:"types,flow"`
	MaxAnimateSizeBytes int64            `yaml:"maxAnimateSizeBytes"`
	Sizes               []*ThumbnailSize `yaml:"sizes,flow"`
	AllowAnimated       bool             `yaml:"allowAnimated"`
	DefaultAnimated     bool             `yaml:"defaultAnimated"`
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
	NumWorkers         int      `yaml:"numWorkers"`
	FilePreviewTypes   []string `yaml:"filePreviewTypes,flow"`
	DisallowedNetworks []string `yaml:"disallowedNetworks,flow"`
	AllowedNetworks    []string `yaml:"allowedNetworks,flow"`
	UnsafeCertificates bool     `yaml:"previewUnsafeCertificates"`
}

type RateLimitConfig struct {
	RequestsPerSecond float64 `yaml:"requestsPerSecond"`
	Enabled           bool    `yaml:"enabled"`
	BurstCount        int     `yaml:"burst"`
}

type IdenticonsConfig struct {
	Enabled bool `yaml:"enabled"`
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

type QuarantineConfig struct {
	ReplaceThumbnails bool   `yaml:"replaceThumbnails"`
	ThumbnailPath     string `yaml:"thumbnailPath"`
	AllowLocalAdmins  bool   `yaml:"allowLocalAdmins"`
}

type MediaRepoConfig struct {
	General     *GeneralConfig      `yaml:"repo"`
	Homeservers []*HomeserverConfig `yaml:"homeservers,flow"`
	Admins      []string            `yaml:"admins,flow"`
	Database    *DatabaseConfig     `yaml:"database"`
	Uploads     *UploadsConfig      `yaml:"uploads"`
	Downloads   *DownloadsConfig    `yaml:"downloads"`
	Thumbnails  *ThumbnailsConfig   `yaml:"thumbnails"`
	UrlPreviews *UrlPreviewsConfig  `yaml:"urlPreviews"`
	RateLimit   *RateLimitConfig    `yaml:"rateLimit"`
	Identicons  *IdenticonsConfig   `yaml:"identicons"`
	Quarantine  *QuarantineConfig   `yaml:"quarantine"`
}

var instance *MediaRepoConfig
var singletonLock = &sync.Once{}
var Path = "media-repo.yaml"

func ReloadConfig() (error) {
	c := NewDefaultConfig()

	// Write a default config if the one given doesn't exist
	_, err := os.Stat(Path)
	exists := err == nil || !os.IsNotExist(err)
	if !exists {
		fmt.Println("Generating new configuration...")
		configBytes, err := yaml.Marshal(c)
		if err != nil {
			return err
		}

		newFile, err := os.Create(Path)
		if err != nil {
			return err
		}

		_, err = newFile.Write(configBytes)
		if err != nil {
			return err
		}

		err = newFile.Close()
		if err != nil {
			return err
		}
	}

	f, err := os.Open(Path)
	if err != nil {
		return err
	}
	defer f.Close()

	buffer, err := ioutil.ReadAll(f)
	err = yaml.Unmarshal(buffer, &c)
	if err != nil {
		return err
	}

	instance = c
	return nil
}

func Get() (*MediaRepoConfig) {
	if instance == nil {
		singletonLock.Do(func() {
			err := ReloadConfig()
			if err != nil {
				panic(err)
			}
		})
	}
	return instance
}

func NewDefaultConfig() *MediaRepoConfig {
	return &MediaRepoConfig{
		General: &GeneralConfig{
			BindAddress:  "127.0.0.1",
			Port:         8000,
			LogDirectory: "logs",
		},
		Database: &DatabaseConfig{
			Postgres: "postgres://your_username:your_password@localhost/database_name?sslmode=disable",
		},
		Homeservers: []*HomeserverConfig{},
		Admins:      []string{},
		Uploads: &UploadsConfig{
			MaxSizeBytes:         104857600, // 100mb
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
			ThumbnailPath:     "",
		},
	}
}
