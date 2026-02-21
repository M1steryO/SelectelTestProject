package loglint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v3"
)

func LoadSettingsFromFile(base Settings, path string) (Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return base, fmt.Errorf("loglint: read config %q: %w", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	var raw any

	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &raw); err != nil {
			return base, fmt.Errorf("loglint: parse json config %q: %w", path, err)
		}
	default:
		// treat everything else as YAML
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return base, fmt.Errorf("loglint: parse yaml config %q: %w", path, err)
		}
	}

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:          "mapstructure",
		WeaklyTypedInput: true,
		Result:           &base,
	})
	if err != nil {
		return base, fmt.Errorf("loglint: init config decoder: %w", err)
	}
	if err := decoder.Decode(raw); err != nil {
		return base, fmt.Errorf("loglint: decode config %q: %w", path, err)
	}

	return base, nil
}
