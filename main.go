package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"

	"github.com/ericchiang/godl/internal/download"
)

func fatalf(format string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, format, v...)
	os.Exit(1)
}

func indent(indent, s string) string {
	s = strings.TrimSpace(s)
	split := strings.Split(s, "\n")
	for i, line := range split {
		split[i] = indent + strings.TrimLeftFunc(line, unicode.IsSpace)
	}
	return strings.Join(split, "\n")
}

type downloader struct {
	disableCache bool
	dir          string
	debug        bool
}

func (d *downloader) project() (*download.Project, error) {
	dir := d.dir
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %v", err)
		}
		dir = cwd
	}

	var cache download.Cache
	if d.disableCache {
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

func main() {
	d := new(downloader)
	c := &cobra.Command{
		Use:   "godl [sub-command]",
		Short: "A Go vendoring tool that allows incremental changes to dependencies.",
		Long: indent("", `
			godl is a vendoring tool that lets users download dependencies one at a
			time. Unlike other tools, it does no inspection of source files in a
			project, reducing the overhead of expensive operations and corner cases.
		`),
	}
	c.AddCommand(cmdDownload(d))

	c.PersistentFlags().BoolVar(&d.disableCache, "disable-cache", false,
		"Disable download cache.")
	c.PersistentFlags().StringVar(&d.dir, "dir", "",
		"Directory to operate in. Defaults to the current directory.")

	if err := c.Execute(); err != nil {
		os.Exit(1)
	}
}

func cmdDownload(d *downloader) *cobra.Command {
	c := &cobra.Command{
		Use:   "download",
		Short: "Download all dependencies for a project to the vendor directory.",
		Long: indent("", `
			Download loads the manifest file for the current project, then attempts to
			fetch all packages and place them in the vendor directory.
		`),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 0 {
				cmd.Usage()
				os.Exit(1)
			}
			p, err := d.project()
			if err != nil {
				fatalf("%v\n", err)
			}
			if err := downloadAll(p); err != nil {
				fatalf("%v\n", err)
			}
		},
	}
	return c
}
