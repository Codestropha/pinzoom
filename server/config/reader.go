package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
)

func ReadYAMLFile(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to open a config file, path=%q, error: %v", path, err)
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("Failed to read a config file, path=%q, error: %v", path, err)
	}

	c := NewConfig()
	if err = yaml.Unmarshal(bytes, c); err != nil {
		return nil, fmt.Errorf("Failed to parse a YAML config file, path=%q, error: %v", path, err)
	}
	return c, nil
}
