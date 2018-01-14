package config

import (
	"io/ioutil"
	"os"
	"sync"

	"gopkg.in/yaml.v2"
)

type HomeserverConfig struct {
	Name                 string `yaml:"name"`
	DownloadRequiresAuth bool   `yaml:"downloadRequiresAuth"`
	ClientServerApi      string `yaml:"csApi"`
}

type MediaRepoConfig struct {
	General struct {
		BindAddress  string `yaml:"bindAddress"`
		Port         int    `yaml:"port"`
		LogDirectory string `yaml:"logDirectory"`
	} `yaml:"repo"`

	Homeservers []HomeserverConfig `yaml:"homeservers,flow"`

	Database struct {
		Postgres string `yaml:"postgres"`
	} `yaml:"database"`

	Uploads struct {
		StoragePaths []string `yaml:"storagePaths,flow"`
		MaxSizeBytes int64    `yaml:"maxBytes"`
	} `yaml:"uploads"`

	Downloads struct {
		MaxSizeBytes int64 `yaml:"maxBytes"`
		NumWorkers   int   `yaml:"numWorkers"`
	} `yaml:"downloads"`

	Thumbnails struct {
		MaxSourceBytes      int64    `yaml:"maxSourceBytes"`
		NumWorkers          int      `yaml:"numWorkers"`
		Types               []string `yaml:"types,flow"`
		MaxAnimateSizeBytes int64    `yaml:"maxAnimateSizeBytes"`
		Sizes []struct {
			Width  int    `yaml:"width"`
			Height int    `yaml:"height"`
			Method string `yaml:"method"`
		} `yaml:"sizes,flow"`
	} `yaml:"thumbnails"`

	UrlPreviews struct {
		Enabled            bool     `yaml:"enabled"`
		MaxPageSizeBytes   int64    `yaml:"maxPageSizeBytes"`
		NumWorkers         int      `yaml:"numWorkers"`
		DisallowedNetworks []string `yaml:"disallowedNetworks,flow"`
		AllowedNetworks    []string `yaml:"allowedNetworks,flow"`
	} `yaml:"urlPreviews"`

	RateLimit struct {
		Enabled bool `yaml:"enabled"`
		// TODO: Support floats when this is fixed: https://github.com/didip/tollbooth/issues/58
		RequestsPerSecond int64 `yaml:"requestsPerSecond"`
		BurstCount        int   `yaml:"burst"`
	} `yaml:"rateLimit"`

	Identicons struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"identicons"`
}

var instance *MediaRepoConfig
var singletonLock = &sync.Once{}

func LoadConfig() (error) {
	c := &MediaRepoConfig{}

	f, err := os.Open("media-repo.yaml")
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
			err := LoadConfig()
			if err != nil {
				panic(err)
			}
		})
	}
	return instance
}
