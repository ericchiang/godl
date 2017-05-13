package cmd

import (
	"fmt"
	"log"
	"sort"

	"github.com/ericchiang/godl/internal/download"
)

func downloadAll(p *download.Project, logger *log.Logger) error {
	m, err := p.LoadManifest()
	if err != nil {
		return err
	}
	l, err := p.LoadLock()
	if err != nil {
		return err
	}

	downloaded := make(map[string]download.LockPackage)
	for _, pkg := range l.Import {
		downloaded[pkg.Package] = pkg
	}

	didSomething := false

	inManifest := make(map[string]struct{})
	for _, pkg := range m.Import {
		inManifest[pkg.Package] = struct{}{}

		lockPkg, ok := downloaded[pkg.Package]
		if ok && packagesEq(lockPkg, pkg) {
			continue
		}
		didSomething = true

		logger.Printf("vendoring %s", pkg.Package)
		lp, err := p.Download(pkg)
		if err != nil {
			return fmt.Errorf("download package %s: %v", pkg.Package, err)
		}
		err = p.UpdateLock(func(l *download.Lock) error {
			for i, lockPkg := range l.Import {
				if lockPkg.Package == lp.Package {
					l.Import[i] = lp
					return nil
				}
			}
			l.Import = append(l.Import, lp)
			return nil
		})
		if err != nil {
			return err
		}
	}

	for _, pkg := range l.Import {
		if _, ok := inManifest[pkg.Package]; !ok {
			didSomething = true
			logger.Printf("removing %s", pkg.Package)
			if err := p.Remove(pkg.Package); err != nil {
				return err
			}
		}
	}

	if !didSomething {
		logger.Printf("dependencies up to date")
	}

	return nil
}

func packagesEq(l download.LockPackage, m download.ManifestPackage) bool {
	if l.Package != m.Package ||
		l.Version != m.Version ||
		l.Remote != m.Remote ||
		len(l.Subpackages) != len(m.Subpackages) {

		return false
	}
	sort.Strings(l.Subpackages)
	sort.Strings(m.Subpackages)
	for i, p := range l.Subpackages {
		if m.Subpackages[i] != p {
			return false
		}
	}
	return true
}
