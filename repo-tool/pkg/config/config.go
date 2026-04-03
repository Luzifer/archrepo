package config

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"go.yaml.in/yaml/v3"
)

type (
	// File represents the configuration file on disk
	File struct {
		Packages map[string]Package `yaml:"packages"`
	}

	// Package defines where to find the sources and what to depend on
	Package struct {
		Repo      string   `yaml:"repo"`
		DependsOn []string `yaml:"dependsOn"`
	}
)

// Load opens and loads a config file from disk
func Load(filename string) (f File, err error) {
	fp, err := os.Open(filename) //#nosec:G304 // Intended to open arbitrary files
	if err != nil {
		return f, fmt.Errorf("opening config: %w", err)
	}
	defer func() {
		if err := fp.Close(); err != nil {
			logrus.WithError(err).Error("closing config")
		}
	}()

	if err = yaml.NewDecoder(fp).Decode(&f); err != nil {
		return f, fmt.Errorf("decoding config: %w", err)
	}

	if err = f.validate(); err != nil {
		return f, fmt.Errorf("validating config: %w", err)
	}

	return f, nil
}
