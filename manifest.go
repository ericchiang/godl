package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
)

const (
	manifestFile = ".godl.json"

	currentVersion = 2
)

type manifest struct {
	Version int   `json:"version"`
	Import  []pkg `json:"import,omitempty"`
}

type pkg struct {
	Package string `json:"package"`
	Version string `json:"version"`
	Remote  string `json:"remote,omitempty"`
}

func loadManifest(path string) (*manifest, bool, error) {
	p := filepath.Join(path, manifestFile)
	return load(p)
}

func initManifest(path string) error {
	p := filepath.Join(path, manifestFile)
	if _, err := os.Stat(p); err == nil {
		return fmt.Errorf("manifest file already exists")
	}
	return write(p, &manifest{})
}

func updateManifest(path string, f func(m *manifest) error) error {
	p := filepath.Join(path, manifestFile)
	m, ok, err := load(p)
	if err != nil {
		return fmt.Errorf("load manifest: %v", err)
	}
	if !ok {
		// manifest should already be created if we're updating it.
		return fmt.Errorf("failed to stat %s", p)
	}

	if m.Version > currentVersion {
		return fmt.Errorf("manifest version (%d) is greater than godl version (%d), please upgrade", m.Version, currentVersion)
	}
	if err := f(m); err != nil {
		return err
	}
	return write(p, m)
}

func load(path string) (*manifest, bool, error) {
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		return nil, false, nil
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read file %s: %v", path, err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, false, fmt.Errorf("failed to parse file %s: %v", path, err)
	}
	return &m, true, nil
}

func write(path string, m *manifest) error {
	m.Version = currentVersion
	sort.Slice(m.Import, func(i, j int) bool {
		return m.Import[i].Package < m.Import[j].Package
	})
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := ioutil.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write file: %v", err)
	}
	return nil
}
