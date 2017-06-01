package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"github.com/ericchiang/godl/internal/download"
	"github.com/ericchiang/godl/internal/forked/glideutil"
)

func importManifest(p *download.Project, logger *log.Logger, manifest string) error {
	data, err := ioutil.ReadFile(manifest)
	if err != nil {
		return fmt.Errorf("read manifest: %v", err)
	}
	var godeps struct {
		Deps []struct {
			ImportPath string
			Rev        string
			Comment    string
		}
	}
	if err := json.Unmarshal(data, &godeps); err != nil {
		return fmt.Errorf("parsing manifest: %v", err)
	}

	var pkgs []download.ManifestPackage
	for _, dep := range godeps.Deps {
		rootPkg, err := glideutil.GetRootFromPackage(dep.ImportPath)
		if err != nil {
			return err
		}

		subPkg := strings.TrimPrefix(strings.TrimPrefix(dep.ImportPath, rootPkg), "/")

		found := false
		for i, pkg := range pkgs {
			if pkg.Package != rootPkg {
				continue
			}

			found = true
			if subPkg == "" {
				// Root package, nothing to do.
				break
			}

			pkgs[i].Subpackages = append(pkg.Subpackages, subPkg)
		}

		if found {
			continue
		}

		version := dep.Rev
		if strings.HasPrefix(dep.Comment, "v") {
			// Comment looks like a version tag.
			version = dep.Comment
		}
		pkg := download.ManifestPackage{
			Package: rootPkg,
			Version: version,
		}
		if subPkg != "" {
			pkg.Subpackages = []string{subPkg}
		}

		logger.Printf("found dependency %s at version %s", pkg.Package, pkg.Version)

		pkgs = append(pkgs, pkg)
	}

	return p.Import(&download.Manifest{Import: pkgs})
}
