package download

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"github.com/ghodss/yaml"
)

const (
	manifestFile = "godl.yaml"
	lockFile     = "godl.lock"
)

// Manifest is the manifest file serialization format.
type Manifest struct {
	Import []ManifestPackage `json:"import,omitempty"`
}

// ManifestPackage is the manifest file serialization of a package.
type ManifestPackage struct {
	Package     string   `json:"package"`
	Version     string   `json:"version"`
	Remote      string   `json:"remote,omitempty"`
	Subpackages []string `json:"subpackages,omitempty"`
}

// Lock is the lock file serialization format.
type Lock struct {
	Import []LockPackage `json:"import"`
}

// LockPackage is the lock file serialization of a package.
type LockPackage struct {
	Package     string   `json:"name"`
	Version     string   `json:"version"`
	Remote      string   `json:"remote,omitempty"`
	Subpackages []string `json:"subpackage,omitempty"`
}

// Project can be used to manage manifest and lock files.
type Project struct {
	// Directory to operate in.
	Dir string

	Cache Cache
}

// Import sets the manifest file. It must not already exist.
func (p *Project) Import(m *Manifest) error {
	path := filepath.Join(p.Dir, manifestFile)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("manifest file already exist")
	}
	return write(path, m)
}

// LoadManifest reads and parses the project's manifest file.
func (p *Project) LoadManifest() (*Manifest, error) {
	var m Manifest
	return &m, load(filepath.Join(p.Dir, manifestFile), &m)
}

// LoadLock reads and parses the project's lock file. If it doesn't exist, an empty
// lock file is returned.
func (p *Project) LoadLock() (*Lock, error) {
	var l Lock
	path := filepath.Join(p.Dir, lockFile)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return &l, nil
		}
		return nil, err
	}
	return &l, load(path, &l)
}

// UpdateLock reads the lock file, appies the passed function, then writes the result.
func (p *Project) UpdateLock(f func(l *Lock) error) error {
	l, err := p.LoadLock()
	if err != nil {
		return err
	}
	if err := f(l); err != nil {
		return err
	}
	for i, p := range l.Import {
		sort.Strings(p.Subpackages)
		l.Import[i] = p
	}
	sort.Slice(l.Import, func(i, j int) bool {
		return l.Import[i].Package < l.Import[j].Package
	})
	return write(filepath.Join(p.Dir, lockFile), l)
}

func load(filepath string, i interface{}) error {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, i)
}

func write(filepath string, i interface{}) error {
	data, err := yaml.Marshal(i)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath, data, 0644)
}
