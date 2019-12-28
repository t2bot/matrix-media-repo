package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"sync"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type runtimeConfig struct {
	MigrationsPath string
	TemplatesPath  string
}

var Runtime = &runtimeConfig{}
var Path = "media-repo.yaml"

var instance *MainRepoConfig
var singletonLock = &sync.Once{}
var domains = make(map[string]*DomainRepoConfig)

func reloadConfig() (*MainRepoConfig, map[string]*DomainRepoConfig, error) {
	c := NewDefaultMainConfig()
	domainConfs := make(map[string]*DomainRepoConfig)

	// Write a default config if the one given doesn't exist
	info, err := os.Stat(Path)
	exists := err == nil || !os.IsNotExist(err)
	if !exists {
		fmt.Println("Generating new configuration...")
		configBytes, err := yaml.Marshal(c)
		if err != nil {
			return nil, nil, err
		}

		newFile, err := os.Create(Path)
		if err != nil {
			return nil, nil, err
		}

		_, err = newFile.Write(configBytes)
		if err != nil {
			return nil, nil, err
		}

		err = newFile.Close()
		if err != nil {
			return nil, nil, err
		}
	}

	// Get new info about the possible directory after creating
	info, err = os.Stat(Path)
	if err != nil {
		return nil, nil, err
	}

	pathsOrdered := make([]string, 0)
	if info.IsDir() {
		logrus.Info("Config is a directory - loading all files over top of each other")

		files, err := ioutil.ReadDir(Path)
		if err != nil {
			return nil, nil, err
		}

		for _, f := range files {
			pathsOrdered = append(pathsOrdered, path.Join(Path, f.Name()))
		}

		sort.Strings(pathsOrdered)
	} else {
		pathsOrdered = append(pathsOrdered, Path)
	}

	pendingDomainConfigs := make(map[string][][]byte)

	for _, p := range pathsOrdered {
		logrus.Info("Loading config file: ", p)

		s, err := os.Stat(p)
		if err != nil {
			return nil, nil, err
		}
		if s.IsDir() {
			continue // skip directories
		}

		f, err := os.Open(p)
		if err != nil {
			return nil, nil, err
		}

		//noinspection GoDeferInLoop
		defer f.Close()

		buffer, err := ioutil.ReadAll(f)
		if err != nil {
			return nil, nil, err
		}

		testMap := make(map[string]interface{})
		err = yaml.Unmarshal(buffer, &testMap)
		if err != nil {
			return nil, nil, err
		}

		if hsRaw, ok := testMap["homeserver"]; ok {
			if hs, ok := hsRaw.(string); ok {
				if _, ok = pendingDomainConfigs[hs]; !ok {
					pendingDomainConfigs[hs] = make([][]byte, 0)
				}
				pendingDomainConfigs[hs] = append(pendingDomainConfigs[hs], buffer)
				continue // skip parsing - we'll do this in a moment
			}
		}

		// Not a domain config - parse into regular config
		err = yaml.Unmarshal(buffer, &c)
		if err != nil {
			return nil, nil, err
		}
	}

	newDomainConfig := func() DomainRepoConfig {
		dc := NewDefaultDomainConfig()
		dc.DataStores = c.DataStores
		dc.Archiving = c.Archiving
		dc.Uploads = c.Uploads
		dc.Identicons = c.Identicons
		dc.Quarantine = c.Quarantine
		dc.TimeoutSeconds = c.TimeoutSeconds
		dc.Downloads = c.Downloads.DownloadsConfig
		dc.Thumbnails = c.Thumbnails.ThumbnailsConfig
		dc.UrlPreviews = c.UrlPreviews.UrlPreviewsConfig
		return dc
	}

	// Start building domain configs
	for _, d := range c.Homeservers {
		dc := newDomainConfig()
		domainConfs[d.Name] = &dc
		domainConfs[d.Name].Name = d.Name
		domainConfs[d.Name].ClientServerApi = d.ClientServerApi
		domainConfs[d.Name].BackoffAt = d.BackoffAt
		domainConfs[d.Name].AdminApiKind = d.AdminApiKind
	}
	for hs, bs := range pendingDomainConfigs {
		if _, ok := domainConfs[hs]; !ok {
			dc := newDomainConfig()
			domainConfs[hs] = &dc
			domainConfs[hs].Name = hs
		}

		for _, b := range bs {
			err = yaml.Unmarshal(b, domainConfs[hs])
			if err != nil {
				return nil, nil, err
			}
		}

		// For good measure...
		domainConfs[hs].Name = hs
	}

	return &c, domainConfs, nil
}

func Get() *MainRepoConfig {
	if instance == nil {
		singletonLock.Do(func() {
			c, d, err := reloadConfig()
			if err != nil {
				logrus.Fatal(err)
			}
			instance = c
			domains = d
		})
	}
	return instance
}

func AllDomains() []*DomainRepoConfig {
	vals := make([]*DomainRepoConfig, 0)
	for _, v := range domains {
		vals = append(vals, v)
	}
	return vals
}

func GetDomain(domain string) *DomainRepoConfig {
	Get() // Ensure we generate a main config
	return domains[domain]
}

func UniqueDatastores() []DatastoreConfig {
	confs := make([]DatastoreConfig, 0)

	for _, dsc := range Get().DataStores {
		confs = append(confs, dsc)
	}

	for _, d := range AllDomains() {
		for _, dsc := range d.DataStores {
			found := false
			for _, edsc := range confs {
				if edsc.Type == dsc.Type {
					if dsc.Type == "file" && edsc.Options["path"] == dsc.Options["path"] {
						found = true
						break
					} else if dsc.Type == "s3" && edsc.Options["endpoint"] == dsc.Options["endpoint"] && edsc.Options["bucketName"] == dsc.Options["bucketName"] {
						found = true
						break
					}
				}
			}
			if found {
				continue
			}
			confs = append(confs, dsc)
		}
	}

	return confs
}

func PrintDomainInfo() {
	logrus.Info("Domains loaded:")
	for _, d := range domains {
		logrus.Info(fmt.Sprintf("\t%s (%s)", d.Name, d.ClientServerApi))
	}
}
