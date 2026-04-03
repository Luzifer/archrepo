package cache

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"go.yaml.in/yaml/v3"
)

const yamlIndent = 2

type (
	// BuildStatus tells how the build went
	BuildStatus uint8

	// File represents the cache on disk
	File struct {
		Packages map[string]Package `yaml:"packages"`

		storagePath string
	}

	// Package stores build-information for the last build
	Package struct {
		Repo            string      `yaml:"repo"`
		LastSeenCommit  string      `yaml:"lastSeenCommit"`
		LastBuiltAt     time.Time   `yaml:"lastBuiltAt"`
		LastBuiltCommit string      `yaml:"lastBuiltCommit"`
		LastBuiltStatus BuildStatus `yaml:"lastBuiltStatus"`
	}
)

// Enum of possible BuildStatus
const (
	BuildStatusUnknown BuildStatus = iota
	BuildStatusSuccess
	BuildStatusBlocked
	BuildStatusFailed
)

// Load reads a cache file from disk and decodes its YAML contents.
func Load(filename string) (f *File, err error) {
	f = &File{
		Packages:    make(map[string]Package),
		storagePath: filename,
	}

	fp, err := os.Open(filename) //#nosec:G304 // Intended to open arbitrary files
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return f, nil
		}

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

	return f, nil
}

// Save writes the cache file to disk using a temporary file and atomic rename.
func (f File) Save() (err error) {
	dir := filepath.Dir(f.storagePath)
	tmpFile, err := os.CreateTemp(dir, filepath.Base(f.storagePath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	tmpName := tmpFile.Name()
	defer func() {
		if err == nil {
			return
		}

		if removeErr := os.Remove(tmpName); removeErr != nil && !os.IsNotExist(removeErr) {
			logrus.WithError(removeErr).WithField("file", tmpName).Error("removing temp cache file")
		}
	}()

	enc := yaml.NewEncoder(tmpFile)
	enc.SetIndent(yamlIndent)

	if err = enc.Encode(f); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("encoding cache: %w", err)
	}

	if err = tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err = os.Rename(tmpName, f.storagePath); err != nil {
		return fmt.Errorf("moving temp file into place: %w", err)
	}

	return nil
}
