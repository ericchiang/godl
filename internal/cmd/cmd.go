// Package cmd implements the command line interface for the godl tool.
package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/ericchiang/godl/internal/download"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
)

func indent(indent, s string) string {
	s = strings.TrimSpace(s)
	split := strings.Split(s, "\n")
	for i, line := range split {
		split[i] = indent + strings.TrimLeftFunc(line, unicode.IsSpace)
	}
	return strings.Join(split, "\n")
}

type options struct {
	disableCache bool
	dir          string
	debug        bool
}

func (o *options) project() (*download.Project, error) {
	dir := o.dir
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %v", err)
		}
		dir = cwd
	}

	var cache download.Cache
	if o.disableCache {
		cache = download.NoCache
	} else {
		home, err := homedir.Dir()
		if err != nil {
			return nil, fmt.Errorf("could not find home directory: %v", err)
		}
		cache = download.NewCache(filepath.Join(home, ".godl"))
	}
	return &download.Project{Dir: dir, Cache: cache}, nil
}

// New returns a new instance of the godl command.
func New() *cobra.Command {
	o := new(options)
	l := log.New(os.Stderr, "", 0)
	c := &cobra.Command{
		Use:   "godl [sub-command]",
		Short: "A Go vendoring tool that allows incremental changes to dependencies.",
		Long: indent("", `
			godl is a vendoring tool that lets users download dependencies one at a
			time. Unlike other tools, it does no inspection of source files in a
			project, reducing the overhead of expensive operations and corner cases.
		`),
	}
	c.AddCommand(cmdVendor(o, l))
	c.AddCommand(cmdImport(o, l))

	c.PersistentFlags().BoolVar(&o.disableCache, "disable-cache", false,
		"Disable download cache.")
	c.PersistentFlags().StringVar(&o.dir, "dir", "",
		"Directory to operate in. Defaults to the current directory.")
	c.PersistentFlags().BoolVarP(&o.debug, "verbose", "v", false,
		"Enable verbose logging.")

	return c
}

func cmdVendor(o *options, l *log.Logger) *cobra.Command {
	c := &cobra.Command{
		Use:   "vendor",
		Short: "Download dependencies to the vendor directory",
		Long: indent("", `
			Load the manifest file and compare it against the lock file for any dependencies
			that need downloading, removal, or updating. Dependencies are then modified one
			at a time.
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return fmt.Errorf("surplus arguments")
			}
			p, err := o.project()
			if err != nil {
				return err
			}
			return downloadAll(p, l)
		},
	}
	return c
}

func cmdImport(o *options, l *log.Logger) *cobra.Command {
	c := &cobra.Command{
		Use:   "import",
		Short: "Import dependencies from an existing package management file",
		Example: indent("  ", `
			godl import Godeps/Godeps.json
			godl import glide.yaml
		`),
		Long: indent("", `
			Inspect an existing manifest file from another package manager. Supported
			tools are godeps, glide, and gvt.
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("import command requires argument")
			}

			p, err := o.project()
			if err != nil {
				return err
			}
			return importManifest(p, l, args[0])
		},
	}
	return c
}
