package download

import (
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
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

		if info.IsDir() {
			return filepath.SkipDir
		}

		if ignore(info) {
			return nil
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		return copyFile(filepath.Join(dest, rel), path, info)
	})
}

func walkImports(pkg string, pkgPath func(pkgName string) string, visit func(pkg string) (bool, error)) error {
	ok, err := visit(pkg)
	if err != nil || !ok {
		return err
	}

	dir := pkgPath(pkg)

	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read dir: %v", err)
	}

	for _, info := range infos {
		if info.IsDir() || !isGoFile(info.Name()) {
			continue
		}
		imports, err := listImports(filepath.Join(dir, info.Name()))
		if err != nil {
			return fmt.Errorf("determining imports: %v", err)
		}

		for _, importedPkg := range imports {
			if err := walkImports(importedPkg, pkgPath, visit); err != nil {
				return err
			}
		}
	}
	return nil
}

// copySubpackages recursively follows subpackage imports as long as
// the import is within the package.
func copySubpackages(dest, pkgRoot string, p ManifestPackage) ([]string, error) {
	visitedPkgs := make(map[string]bool)

	absPath := func(root, pkgName string) string {
		relImport := strings.TrimPrefix(pkgName, p.Package)
		return filepath.Join(root, filepath.FromSlash(relImport))
	}

	pkgPath := func(pkgName string) string { return absPath(pkgRoot, pkgName) }
	destPath := func(pkgName string) string { return absPath(dest, pkgName) }

	visit := func(pkg string) (bool, error) {
		if visitedPkgs[pkg] {
			// Already visited this package.
			return false, nil
		}

		if !strings.HasPrefix(pkg, p.Package) {
			// Package is outside of this repo. Don't follow the imports.
			return false, nil
		}

		if ok, err := isMain(pkgPath(pkg)); err != nil || ok {
			return false, err
		}

		if err := copyDir(destPath(pkg), pkgPath(pkg)); err != nil {
			return false, err
		}

		seen := visitedPkgs[pkg]
		visitedPkgs[pkg] = true
		return !seen, nil
	}

	toVisit := make([]string, len(p.Subpackages)+1)
	toVisit[0] = p.Package
	for i, relImport := range p.Subpackages {
		toVisit[i+1] = path.Join(p.Package, relImport)
	}

	for _, pkg := range toVisit {
		if err := walkImports(pkg, pkgPath, visit); err != nil {
			return nil, err
		}
	}

	var subPackages []string

	for pkg := range visitedPkgs {
		relImport := strings.TrimPrefix(pkg, p.Package)
		relImport = strings.TrimPrefix(relImport, "/")

		if relImport == "" {
			// Root package. Continue
			continue
		}
		subPackages = append(subPackages, relImport)
	}
	sort.Strings(subPackages)
	return subPackages, nil
}

func isMain(pkgPath string) (bool, error) {
	infos, err := ioutil.ReadDir(pkgPath)
	if err != nil {
		return false, err
	}

	for _, info := range infos {
		if info.IsDir() {
			continue
		}
		if !isGoFile(info.Name()) {
			continue
		}

		p := filepath.Join(pkgPath, info.Name())

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, p, nil, parser.PackageClauseOnly)
		if err != nil {
			return false, fmt.Errorf("parse file %s: %v", p, err)
		}

		if f.Name.Name != "main" {
			return false, nil
		}
	}
	return true, nil
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
			imports = append(imports,
				strings.TrimSuffix(
					strings.TrimPrefix(i.Path.Value, `"`), `"`))
		}
	}
	return
}
