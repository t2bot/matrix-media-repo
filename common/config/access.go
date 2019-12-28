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
var domains = make(map[string]DomainRepoConfig)

func reloadConfig() (*MainRepoConfig, error) {
	c := NewDefaultMainConfig()

	// Write a default config if the one given doesn't exist
	info, err := os.Stat(Path)
	exists := err == nil || !os.IsNotExist(err)
	if !exists {
		fmt.Println("Generating new configuration...")
		configBytes, err := yaml.Marshal(c)
		if err != nil {
			return nil, err
		}

		newFile, err := os.Create(Path)
		if err != nil {
			return nil, err
		}

		_, err = newFile.Write(configBytes)
		if err != nil {
			return nil, err
		}

		err = newFile.Close()
		if err != nil {
			return nil, err
		}
	}

	// Get new info about the possible directory after creating
	info, err = os.Stat(Path)
	if err != nil {
		return nil, err
	}

	pathsOrdered := make([]string, 0)
	if info.IsDir() {
		logrus.Info("Config is a directory - loading all files over top of each other")

		files, err := ioutil.ReadDir(Path)
		if err != nil {
			return nil, err
		}

		for _, f := range files {
			pathsOrdered = append(pathsOrdered, path.Join(Path, f.Name()))
		}

		sort.Strings(pathsOrdered)
	} else {
		pathsOrdered = append(pathsOrdered, Path)
	}

	for _, p := range pathsOrdered {
		logrus.Info("Loading config file: ", p)
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}

		//noinspection GoDeferInLoop
		defer f.Close()

		buffer, err := ioutil.ReadAll(f)
		err = yaml.Unmarshal(buffer, &c)
		if err != nil {
			return nil, err
		}
	}

	return &c, nil
}

func Get() *MainRepoConfig {
	if instance == nil {
		singletonLock.Do(func() {
			c, err := reloadConfig()
			if err != nil {
				logrus.Fatal(err)
			}
			instance = c
		})
	}
	return instance
}
