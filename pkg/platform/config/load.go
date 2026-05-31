package config

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const DefaultPath = "config/environments.yaml"

func LoadFile(path string) (EnvironmentCatalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return EnvironmentCatalog{}, fmt.Errorf("read environment catalog %q: %w", path, err)
	}

	return Load(data)
}

func Load(data []byte) (EnvironmentCatalog, error) {
	var catalog EnvironmentCatalog
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	if err := decoder.Decode(&catalog); err != nil {
		return EnvironmentCatalog{}, fmt.Errorf("decode environment catalog: %w", err)
	}

	return catalog, nil
}
