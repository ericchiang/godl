package download

import (
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

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

func copyFile(dest, src string, info os.FileInfo) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	destF, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode())
	if err != nil {
		return err
	}
	defer destF.Close()

	f, err := os.OpenFile(src, os.O_RDONLY, info.Mode())
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(destF, f)
	return err
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

		return copyFile(filepath.Join(dest, rel), path, info)
	})
}

// copySubpackages recursively follows subpackage imports as long as
// the import is within the package.
func copySubpackages(dest, pkgRoot string, p ManifestPackage) ([]string, error) {
	visited := make(map[string]bool)
	var seen []string

	var visit func(string) error
	visit = func(rel string) error {
		if visited[rel] {
			return nil
		}
		seen = append(seen, filepath.ToSlash(rel))

		visited[rel] = true

		infos, err := ioutil.ReadDir(filepath.Join(pkgRoot, rel))
		if err != nil {
			return err
		}
		for _, info := range infos {
			if info.IsDir() {
				continue
			}
			if ignore(info) {
				continue
			}

			name := info.Name()
			from := filepath.Join(pkgRoot, rel, name)
			to := filepath.Join(dest, rel, name)

			// If the file is a Go file, analyize its imports and
			// try to follow them.
			if filepath.Ext(name) == ".go" {
				imports, err := listImports(from)
				if err != nil {
					return err
				}
				for _, pkg := range imports {
					if !strings.HasPrefix(pkg, p.Package) {
						// import isn't part of this package
						continue
					}

					// Convert package name into a path relative to
					// the root directory.
					rel := strings.TrimPrefix(pkg, p.Package)
					rel = strings.TrimPrefix(rel, "/")
					rel = filepath.FromSlash(rel)
					if err := visit(rel); err != nil {
						return err
					}
				}
			}
			if err := copyFile(to, from, info); err != nil {
				return err
			}
		}
		return nil
	}

	for _, pkg := range p.Subpackages {
		if err := visit(filepath.FromSlash(pkg)); err != nil {
			return nil, err
		}
	}
	return seen, nil
}

// listImports parses a .go file and returns its imports.
func listImports(filepath string) (imports []string, err error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath, nil, parser.ImportsOnly)
	if err != nil {
		return nil, fmt.Errorf("parse file %s: %v", filepath, err)
	}
	for _, i := range f.Imports {
		if i.Path != nil {
			imports = append(imports, i.Path.Value)
		}
	}
	return
}
