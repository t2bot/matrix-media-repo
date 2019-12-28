package config

import (
	"gopkg.in/yaml.v2"
)

func mapToObjYaml(input map[string]interface{}, ref interface{}) error {
	encoded, err := yaml.Marshal(input)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(encoded, ref)
	return err
}

func objToMapYaml(input interface{}) (map[string]interface{}, error) {
	encoded, err := yaml.Marshal(input)
	if err != nil {
		return nil, err
	}

	m := make(map[string]interface{})
	err = yaml.Unmarshal(encoded, &m)
	return m, err
}
