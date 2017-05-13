package main

import (
	"fmt"
	"os"

	"github.com/ericchiang/godl/internal/cmd"
)

func main() {
	if err := cmd.New().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
