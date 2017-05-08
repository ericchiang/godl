package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestIsEmptyDir(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if empty, err := isEmptyDir(dir); err != nil {
		t.Errorf("isEmptyDir(tempDir): %v", err)
	} else if !empty {
		t.Errorf("expected directory to be empty")
	}

	if err := os.Mkdir(filepath.Join(dir, "foo"), 0755); err != nil {
		t.Fatal(err)
	}

	if empty, err := isEmptyDir(dir); err != nil {
		t.Errorf("isEmptyDir(tempDir): %v", err)
	} else if empty {
		t.Errorf("expected directory to be not empty")
	}
}
