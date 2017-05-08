package main

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/vcs"
	"github.com/ericchiang/godl/internal/forked/glideutil"
)

var errNoManifest = errors.New("manifest file not found, run 'godl init' to create one")

type downloader struct {
	dir   string
	cache cache
}

func newRepo(typ vcs.Type, remote, local string) (vcs.Repo, error) {
	switch typ {
	case vcs.Git:
		return vcs.NewGitRepo(remote, local)
	case vcs.Svn:
		return vcs.NewSvnRepo(remote, local)
	case vcs.Bzr:
		return vcs.NewBzrRepo(remote, local)
	case vcs.Hg:
		return vcs.NewHgRepo(remote, local)
	default:
		return vcs.NewRepo(remote, local)
	}
}

// get downloads a package to a project's "vendor" directory.
//
// Remote and version are both optional.
func (d *downloader) get(pkgName, version, remote string) error {
	m, ok, err := loadManifest(d.dir)
	if err != nil {
		return fmt.Errorf("loading manifest: %v", err)
	}
	if !ok {
		return errNoManifest
	}

	rootPkg, err := glideutil.GetRootFromPackage(pkgName)
	if err != nil {
		return fmt.Errorf("failed to determine root package: %v", err)
	}
	if rootPkg != pkgName {
		return fmt.Errorf("package %q is not the root package import, try downloading %q instead", pkgName, rootPkg)
	}

	// If a remote wasn't specified, but was previously, use that.
	if remote == "" {
		for _, pkg := range m.Import {
			if pkg.Package == pkgName {
				remote = pkg.Remote
				break
			}
		}
	}

	v, err := d.vendorRepo(vcs.NoVCS, pkgName, version, remote)
	if err != nil {
		return err
	}

	return updateManifest(d.dir, func(m *manifest) error {
		p := pkg{
			Package: pkgName,
			Version: v,
			Remote:  remote,
		}

		for i, pkg := range m.Import {
			if pkg.Package == pkgName {
				m.Import[i] = p
				return nil
			}
		}

		m.Import = append(m.Import, p)
		return nil
	})
}

// vendorRepo is like vendor but allows specifying a VSC type. This is to allow
// tests to reference local git repos.
func (d *downloader) vendorRepo(typ vcs.Type, pkgName, version, remote string) (gotVersion string, err error) {
	if u, err := url.Parse(pkgName); err == nil && u.Scheme != "" {
		return "", fmt.Errorf("%q not allowed in import path", u.Scheme)
	}

	if remote == "" {
		remote = "https://" + pkgName
	}

	dest := filepath.Join(d.dir, "vendor", filepath.FromSlash(pkgName))
	err = d.cache.withLock(remote, func(path string) error {
		repo, err := newRepo(typ, remote, path)
		if err != nil {
			return fmt.Errorf("setting up remote: %v", err)
		}

		gotVersion, err = d.downloadRepo(repo, version)
		if err != nil {
			return fmt.Errorf("downloading repo: %v", err)
		}

		if err := os.RemoveAll(dest); err != nil {
			return fmt.Errorf("clearing existing path in vendor directory: %v", err)
		}

		if err := os.MkdirAll(dest, 0755); err != nil {
			return fmt.Errorf("creating target directory: %v", err)
		}

		if err := copyDir(dest, path); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return
}

// downloadRepo attempts to download a repo at a given version. The repo may already be
// cloned, or not.
func (d *downloader) downloadRepo(repo vcs.Repo, version string) (string, error) {
	if !repo.CheckLocal() {
		if err := repo.Get(); err != nil {
			if e, ok := err.(*vcs.RemoteError); ok {
				return "", fmt.Errorf("%s: %s %v", e.Error(), e.Out(), e.Original())
			}
			return "", fmt.Errorf("getting repo: %v", err)
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
		return "", fmt.Errorf("failed to update to verison %s of repo: %v", version, err)
	}
	return version, nil
}

func ignore(info os.FileInfo) bool {
	if info.IsDir() {
		switch info.Name() {
		case "testdata", "vendor":
			return true
		}
		return strings.HasPrefix(info.Name(), ".")
	}

	// Ignore non-normal files (e.g. symlink).
	if info.Mode()&os.ModeType != 0 {
		return true
	}

	switch filepath.Ext(info.Name()) {
	case ".go":
		// Ignore test files.
		if strings.HasSuffix(info.Name(), "_test.go") {
			return true
		}
	case ".s", ".c":
		// Retain assembly and C files.
	default:
		return info.Name() != "LICENSE" && info.Name() != "LICENSE.txt"
	}
	return false
}

func copyDir(dest, src string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil || src == path {
			return err
		}

		if ignore(info) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		target := filepath.Join(dest, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		destF, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode())
		if err != nil {
			return err
		}
		defer destF.Close()

		f, err := os.OpenFile(path, os.O_RDONLY, info.Mode())
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(destF, f)
		return err
	})
}
