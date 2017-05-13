package download

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"

	"github.com/Masterminds/vcs"
	"go4.org/lock"

	"github.com/ericchiang/godl/internal/forked/glideutil"
)

// Cache provides a space for downloading packages.
type Cache interface {
	// Dir maps a remote repo to a directory.
	Dir(remote string, f func(dir string) error) error
	// Clear removes all cached packages from disk.
	Clear() error
}

// NewCache returns a repo for a cache.
func NewCache(dir string) Cache { return cacheDir{dir} }

// NoCache is a cache implementation that doesn't cache anything.
var NoCache Cache = tempDir{}

type tempDir struct{}

func (t tempDir) Dir(remote string, f func(dir string) error) error {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	return f(dir)
}

func (t tempDir) Clear() error { return nil }

// cacheDir is a cache implementation that returns returns a new
type cacheDir struct {
	dir string
}

func (c cacheDir) Clear() error {
	return os.RemoveAll(c.dir)
}

func (c cacheDir) Dir(remote string, f func(dir string) error) error {
	h := sha256.New()
	io.WriteString(h, remote)
	hash := hex.EncodeToString(h.Sum(nil))

	dir := filepath.Join(c.dir, "src", hash)

	lockFile := dir + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockFile), 0755); err != nil {
		return err
	}

	closer, err := lock.Lock(lockFile)
	if err != nil {
		return fmt.Errorf("could not create lock file for remote %s, is another process downloading that package? (%v)", remote, err)
	}
	defer closer.Close()

	return f(dir)
}

func (p *Project) packagePath(importPath string) string {
	return filepath.Join(p.Dir, "vendor", filepath.FromSlash(importPath))
}

// Remove deletes a package from the vendor directory of a project.
func (p *Project) Remove(importPath string) error {
	return os.RemoveAll(p.packagePath(importPath))
}

// Download downloads a package to the vendor directory of a project.
// It does not modify the lock files.
func (p *Project) Download(pkg ManifestPackage) (LockPackage, error) {
	l := LockPackage{Package: pkg.Package}
	if u, err := url.Parse(pkg.Package); err == nil && u.Scheme != "" {
		return l, fmt.Errorf("%q not allowed in import path", u.Scheme)
	}

	rootPkg, err := glideutil.GetRootFromPackage(pkg.Package)
	if err != nil {
		return l, fmt.Errorf("failed to determine root package: %v", err)
	}
	if rootPkg != pkg.Package {
		return l, fmt.Errorf("package %s is not the repo's root package, try %s instead", pkg.Package, rootPkg)
	}

	l.Remote = pkg.Remote
	remote := pkg.Remote
	if remote == "" {
		remote = "https://" + pkg.Package
	}

	dest := p.packagePath(pkg.Package)
	err = p.Cache.Dir(remote, func(cachePath string) error {
		repo, err := vcs.NewRepo(remote, cachePath)
		if err != nil {
			return fmt.Errorf("setting up remote: %v", err)
		}
		version, err := downloadRepo(repo, pkg.Version)
		if err != nil {
			return fmt.Errorf("download repo: %v", err)
		}
		l.Version = version
		if err := os.RemoveAll(dest); err != nil {
			return fmt.Errorf("clearing existing path in vendor directory: %v", err)
		}

		if err := os.MkdirAll(dest, 0755); err != nil {
			return fmt.Errorf("creating target directory: %v", err)
		}

		if len(pkg.Subpackages) == 0 {
			if err := copyDir(dest, cachePath); err != nil {
				return fmt.Errorf("copying files: %v", err)
			}
			return nil
		}
		subPkgs, err := copySubpackages(dest, cachePath, pkg)
		if err != nil {
			return fmt.Errorf("copying files: %v", err)
		}
		l.Subpackages = subPkgs
		return nil
	})
	if err != nil {
		return l, err
	}

	return l, nil
}

func downloadRepo(repo vcs.Repo, version string) (string, error) {
	if !repo.CheckLocal() {
		if err := repo.Get(); err != nil {
			if e, ok := err.(*vcs.RemoteError); ok {
				return "", fmt.Errorf("%s: %s %v", e.Error(), e.Out(), e.Original())
			}
		}
	}

	if version != "" {
		if err := repo.UpdateVersion(version); err == nil {
			return version, nil
		}
	}
	if err := repo.Update(); err != nil {
		return "", fmt.Errorf("updaing repo: %v", err)
	}

	if version == "" {
		return repo.Version()
	}
	if err := repo.UpdateVersion(version); err != nil {
		return "", fmt.Errorf("failed to update to version %s of repo: %v", version, err)
	}
	return version, nil
}
