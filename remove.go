package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func remove(projDir, pkgName string) error {
	m, ok, err := loadManifest(projDir)
	if err == nil && !ok {
		err = errNoManifest
	}
	if err != nil {
		return err
	}

	found := false
	for _, pkg := range m.Import {
		if pkg.Package == pkgName {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("package not found in manifest")
	}

	vendorDir := filepath.Join(projDir, "vendor")
	p := filepath.Join(vendorDir, filepath.ToSlash(pkgName))
	if err := os.RemoveAll(p); err != nil {
		return err
	}
	for {
		p = filepath.Dir(p)
		if p == projDir {
			break
		}
		ok, err := isEmptyDir(p)
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		if err := os.Remove(p); err != nil {
			return err
		}
	}

	return updateManifest(projDir, func(m *manifest) error {
		n := 0
		for _, pkg := range m.Import {
			if pkg.Package != pkgName {
				m.Import[n] = pkg
				n++
			}
		}
		m.Import = m.Import[:n]
		return nil
	})
}

func isEmptyDir(dir string) (bool, error) {
	f, err := os.Open(dir)
	if err != nil {
		return false, err
	}

	_, err = f.Readdir(1)
	f.Close()
	if err != nil && err == io.EOF {
		return true, nil
	}
	return false, err
}
