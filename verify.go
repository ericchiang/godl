package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func verify(dir string) (bad []string, err error) {
	m, ok, err := loadManifest(dir)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errNoManifest
	}
	for _, pkg := range m.Import {
		p := filepath.Join(dir, "vendor", filepath.FromSlash(pkg.Package))
		got, err := dirSum(p)
		if err != nil {
			return nil, err
		}
		if got != pkg.Checksum {
			bad = append(bad, pkg.Package)
		}
	}
	return bad, nil
}

// dirSum computes a naive hash of a directory.
func dirSum(dir string) (string, error) {
	h := sha256.New()
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		// Ensure file and directory names haven't change.
		io.WriteString(h, filepath.ToSlash(rel))

		if info.IsDir() {
			return nil
		}

		f, err := os.OpenFile(path, os.O_RDONLY, 0644)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(h, f); err != nil {
			return fmt.Errorf("reading from file %s: %v", path, err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
