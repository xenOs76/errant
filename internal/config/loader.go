package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var defaultCandidateNames = []string{
	"Errantfile.yaml",
	"Errantfile.yml",
	"Errantfile",
	".errant.yaml",
	".errant.yml",
}

// FindConfigFile searches for an Errantfile in the current directory or parents.
func FindConfigFile(explicitPath string) (string, error) {
	if explicitPath != "" {
		if _, err := os.Stat(explicitPath); err != nil {
			return "", fmt.Errorf("configuration file not found: %s", explicitPath)
		}

		return explicitPath, nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		for _, name := range defaultCandidateNames {
			candidate := filepath.Join(dir, name)
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	return "", errors.New("no Errantfile.yaml found in current directory or any parent")
}

// LoadFile reads, validates against JSON schema, and parses an Errantfile.
func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	if err := ValidateYAML(data); err != nil {
		return nil, fmt.Errorf("invalid configuration in %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML in %s: %w", path, err)
	}

	return &cfg, nil
}
