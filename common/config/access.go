package config

import (
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"sync"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type runtimeConfig struct {
	MigrationsPath  string
	TemplatesPath   string
	AssetsPath      string
	IsImportProcess bool
}

const DefaultMigrationsPath = "./migrations"
const DefaultTemplatesPath = "./templates"
const DefaultAssetsPath = "./assets"

var Runtime = &runtimeConfig{}
var Path = "media-repo.yaml"

var instance *MainRepoConfig
var singletonLock = &sync.Once{}
var domains = make(map[string]*DomainRepoConfig)

func reloadConfig() (*MainRepoConfig, map[string]*DomainRepoConfig, error) {
	domainConfs := make(map[string]*DomainRepoConfig)

	// Write a default config if the one given doesn't exist
	_, err := os.Stat(Path)
	exists := err == nil || !os.IsNotExist(err)
	if !exists {
		fmt.Println("Generating new configuration...")
		configBytes, err := yaml.Marshal(NewDefaultMainConfig())
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
	info, err := os.Stat(Path)
	if err != nil {
		return nil, nil, err
	}

	pathsOrdered := make([]string, 0)
	if info.IsDir() {
		logrus.Info("Config is a directory - loading all files over top of each other")

		files, err := os.ReadDir(Path)
		if err != nil {
			return nil, nil, err
		}

		for _, f := range files {
			if f.IsDir() {
				continue
			}
			pathsOrdered = append(pathsOrdered, path.Join(Path, f.Name()))
		}

		sort.Strings(pathsOrdered)
	} else {
		pathsOrdered = append(pathsOrdered, Path)
	}

	// Note: the rest of this relies on maps before finalizing on objects because when
	// the yaml is parsed it causes default values for the types to land in the overridden
	// config. We don't want this, so we use maps which inherently override only what is
	// present then we convert that overtop of a default object we create.
	pendingDomainConfigs := make(map[string][][]byte)
	cMap := make(map[string]interface{})

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
		//goland:noinspection GoDeferInLoop
		defer f.Close()

		buffer, err := io.ReadAll(f)
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
		err = yaml.Unmarshal(buffer, &cMap)
		if err != nil {
			return nil, nil, err
		}
	}

	c := NewDefaultMainConfig()
	err = mapToObjYaml(cMap, &c)
	if err != nil {
		return nil, nil, err
	}

	// Start building domain configs
	dMaps := make(map[string]map[string]interface{})
	for _, d := range c.Homeservers {
		dc := DomainConfigFrom(c)
		dc.Name = d.Name
		dc.ClientServerApi = d.ClientServerApi
		dc.BackoffAt = d.BackoffAt
		dc.AdminApiKind = d.AdminApiKind

		m, err := objToMapYaml(dc)
		if err != nil {
			return nil, nil, err
		}
		dMaps[d.Name] = m
	}
	for hs, bs := range pendingDomainConfigs {
		if _, ok := dMaps[hs]; !ok {
			dc := DomainConfigFrom(c)
			dc.Name = hs

			m, err := objToMapYaml(dc)
			if err != nil {
				return nil, nil, err
			}
			dMaps[hs] = m
		}

		for _, b := range bs {
			m := dMaps[hs]
			err = yaml.Unmarshal(b, &m)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	for hs, m := range dMaps {
		drc := DomainRepoConfig{}
		err = mapToObjYaml(m, &drc)
		if err != nil {
			return nil, nil, err
		}

		// For good measure...
		domainConfs[hs] = &drc
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

func AddDomainForTesting(domain string, config *DomainRepoConfig) {
	Get() // Ensure the "main" config was loaded first
	if config == nil {
		c := NewDefaultDomainConfig()
		config = &c
	}
	domains[domain] = config
}

func DomainConfigFrom(c MainRepoConfig) DomainRepoConfig {
	// HACK: We should be better at this kind of inheritance
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
	dc.AccessTokens = c.AccessTokens
	dc.Features = c.Features
	return dc
}

func UniqueDatastores() []DatastoreConfig {
	confs := make([]DatastoreConfig, 0)
	confs = append(confs, Get().DataStores...)

	for _, d := range AllDomains() {
		for _, dsc := range d.DataStores {
			found := false
			for _, existingDsc := range confs {
				if existingDsc.Id == dsc.Id {
					found = true
					break
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

func CheckDeprecations() {
}
