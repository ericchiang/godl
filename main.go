package main

import (
	"fmt"
	"os"
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
)

func fatalf(format string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, format, v...)
	os.Exit(1)
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
		Use: "godl",
	}
	c.AddCommand(cmdInit(t))
	c.AddCommand(cmdGet(t))
	c.AddCommand(cmdVerify(t))

	c.Flags().BoolVar(&t.disableCache, "disable-cache", false,
		"Disable download cache.")
	c.Flags().StringVar(&t.dir, "dir", "",
		"Directory to operate in. Defaults to the current directory.")

	if err := c.Execute(); err != nil {
		os.Exit(1)
	}

}

func cmdVerify(tool *tool) *cobra.Command {
	c := &cobra.Command{
		Use:   "verify",
		Short: "Verify compares the expected and actual state of the vendor directory.",
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
		Short: "Initializes a manifest file to track dependencies in.",
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
		Use:   "get",
		Short: "Get or update a dependency at a specific version.",
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
