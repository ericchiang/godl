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

func isGoFile(name string) bool {
	return filepath.Ext(name) == ".go" && !strings.HasSuffix(name, "_test.go")
}

func isLicense(name string) bool {
	return name == "LICENSE" || name == "LICENSE.txt"
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

	if isGoFile(info.Name()) {
		return false
	}
	if isLicense(info.Name()) {
		return false
	}

	switch filepath.Ext(info.Name()) {
	case ".s", ".c":
		// Retain assembly and C files.
		return false
	}
	return true
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
	// List of directories already visited. These are always in filepath format.
	visitedDirs := make(map[string]bool)

	// List of subpackages seen. These are always in path format.
	var seenImports []string

	var visit func(string) error
	visit = func(rel string) error {
		if visitedDirs[rel] {
			return nil
		}
		if rel != "" {
			seenImports = append(seenImports, filepath.ToSlash(rel))
		}

		visitedDirs[rel] = true

		infos, err := ioutil.ReadDir(filepath.Join(pkgRoot, rel))
		if err != nil {
			return fmt.Errorf("read dir: %v", err)
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
			if isGoFile(name) {
				imports, err := listImports(from)
				if err != nil {
					return fmt.Errorf("determining imports: %v", err)
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
				return fmt.Errorf("copying %s to %s: %v", from, to, err)
			}
		}
		return nil
	}

	if err := visit(""); err != nil {
		return nil, err
	}
	for _, pkg := range p.Subpackages {
		if err := visit(filepath.FromSlash(pkg)); err != nil {
			return nil, err
		}
	}

	// Find any license files in subdirectories of copied directories.
	for pkg := range visitedDirs {
		dir := pkg
		for {
			dir = filepath.Dir(dir)
			if dir == "." || dir == "" {
				break
			}
			if visitedDirs[dir] {
				break
			}
			visitedDirs[dir] = true

			infos, err := ioutil.ReadDir(filepath.Join(pkgRoot, dir))
			if err != nil {
				return nil, err
			}

			for _, info := range infos {
				name := info.Name()
				if isLicense(name) {
					destPath := filepath.Join(dest, dir, name)
					srcPath := filepath.Join(pkgRoot, dir, name)
					if err := copyFile(destPath, srcPath, info); err != nil {
						return nil, fmt.Errorf("copying license: %v", err)
					}
				}
			}

			// At the package root. We can break now.
			if dir == "." || dir == "" {
				break
			}
		}
	}

	return seenImports, nil
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
