package synapse

import (
	"errors"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

type HomeserverYaml struct {
	Database struct {
		Name string `yaml:"name"`
		Arguments struct {
			Username string `yaml:"user"`
			Password string `yaml:"password"`
			Database string `yaml:"database"`
			Hostname string `yaml:"host"`
		} `yaml:"args"`
	} `yaml:"database"`
}

func ReadConfig(yamlPath string) (HomeserverYaml, error) {
	c := &HomeserverYaml{}

	f, err := os.Open(yamlPath)
	if err != nil {
		return *c, err
	}

	defer f.Close()

	buffer, err := ioutil.ReadAll(f)
	err = yaml.Unmarshal(buffer, &c)
	if err != nil {
		return *c, err
	}

	return *c, nil
}

func (c *HomeserverYaml) GetConnectionString() (string) {
	if c.Database.Name != "psycopg2" {
		panic(errors.New("homeserver database must be postgres"))
	}

	a := c.Database.Arguments

	return "postgres://" + a.Username + ":" + a.Password + "@" + a.Hostname + "/" + a.Database + "?sslmode=disable"
}