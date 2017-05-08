package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
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

type tool struct {
	disableCache bool
	dir          string
	debug        bool
}

func (t *tool) downloader() (*downloader, error) {
	dir, err := t.projectDir()
	if err != nil {
		return nil, err
	}

	if t.disableCache {
		return &downloader{dir, noCache}, nil
	}
	home, err := homedir.Dir()
	if err != nil {
		return nil, fmt.Errorf("could not find home directory: %v\n", err)
	}
	c := cacheDir{dir: filepath.Join(home, ".godl")}
	return &downloader{dir, c}, nil
}

func (t *tool) projectDir() (string, error) {
	if t.dir != "" {
		return t.dir, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %v", err)
	}
	return dir, nil
}

func main() {
	t := new(tool)
	c := &cobra.Command{
		Use:   "godl [sub-command]",
		Short: "A Go vendoring tool that allows incremental changes to dependencies.",
		Long: indent("", `
			godl is a vendoring tool that lets users download dependencies one at a
			time. Unlike other tools, it does no inspection of source files in a
			project, reducing the overhead of expensive operations and corner cases.
		`),
		Example: indent("  ", `
			godl init
			godl get github.com/spf13/cobra d83a1d7ccd00a9e1b5d234653837b498b9b27abd
			godl get gopkg.in/square/go-jose.v2 v2.1.1
			godl get github.com/spf13/pflag
		`),
	}
	c.AddCommand(cmdInit(t))
	c.AddCommand(cmdGet(t))
	c.AddCommand(cmdVerify(t))
	c.AddCommand(cmdRm(t))

	c.PersistentFlags().BoolVar(&t.disableCache, "disable-cache", false,
		"Disable download cache.")
	c.PersistentFlags().StringVar(&t.dir, "dir", "",
		"Directory to operate in. Defaults to the current directory.")

	if err := c.Execute(); err != nil {
		os.Exit(1)
	}
}

func cmdVerify(tool *tool) *cobra.Command {
	c := &cobra.Command{
		Use:   "verify",
		Short: "Verify that godl was used to create the vendor directory",
		Long: indent("", `
			Best effort verification that godl was used to create the vendor directory.
		`),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 0 {
				cmd.Usage()
				os.Exit(1)
			}
			dir, err := tool.projectDir()
			if err != nil {
				fatalf("%v\n", err)
			}
			bad, err := verify(dir)
			if err != nil {
				fatalf("%v\n", err)
			}
			if len(bad) == 0 {
				return
			}
			for _, s := range bad {
				fmt.Fprintf(os.Stderr, "failed to verify package: %s\n", s)
			}
			os.Exit(1)
		},
	}
	return c
}

func cmdInit(tool *tool) *cobra.Command {
	c := &cobra.Command{
		Use:   "init",
		Short: "Initialize a manifest file",
		Long: indent("", `
			Initialize a manifest file. To ensure godl is being used in the correct
			directory, it will not automatically create a manifest file.
		`),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 0 {
				cmd.Usage()
				os.Exit(1)
			}
			dir, err := tool.projectDir()
			if err != nil {
				fatalf("%v\n", err)
			}

			if err := initManifest(dir); err != nil {
				fatalf("%v\n", err)
			}
		},
	}
	return c
}

func cmdGet(tool *tool) *cobra.Command {
	var remote string
	c := &cobra.Command{
		Use:   "get <package> [version]",
		Short: "Get, update or downgrade a dependency",
		Long: indent("", `
			Get, update, or downgrade a dependency.

			If version isn't specified, the latest of the default branch will
			be detected.

			The package MUST be the root of the repo. For example "golang.org/x/net"
			will work but "golang.org/x/net/context" won't.
		`),
		Example: indent("  ", `
			godl get golang.org/x/net feeb485667d1fdabe727840fe00adc22431bc86e
			godl get gopkg.in/square/go-jose.v2 v2.1.0
			godl get github.com/spf13/cobra # Default's to latest
		`),
		Run: func(cmd *cobra.Command, args []string) {
			var pkgPath, version string
			switch len(args) {
			case 1:
				pkgPath = args[0]
			case 2:
				pkgPath = args[0]
				version = args[1]
			default:
				cmd.Usage()
				os.Exit(1)
			}

			dl, err := tool.downloader()
			if err != nil {
				fatalf("%v\n", err)
			}
			if err := dl.vendor(pkgPath, version, remote); err != nil {
				fatalf("%v\n", err)
			}
		},
	}

	c.Flags().StringVar(&remote, "remote", "",
		"Remote address fo the repo to download. Can be used to override discoved address.")
	return c
}

func cmdRm(tool *tool) *cobra.Command {
	c := &cobra.Command{
		Use:   "rm <package>",
		Short: "Remove a vendored package",
		Long: indent("", `
			Remove a package from the vendor directory.
		`),
		Example: indent("  ", `
			godl rm golang.org/x/net
			godl rm github.com/spf13/cobra
		`),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}
			pkgName := args[0]

			dir, err := tool.projectDir()
			if err != nil {
				fatalf("%v\n", err)
			}

			if err := remove(dir, pkgName); err != nil {
				fatalf("%v\n", err)
			}
		},
	}
	return c
}
