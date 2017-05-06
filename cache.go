package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"go4.org/lock"
)

type cache interface {
	withLock(remote string, f func(dir string) error) error
}

var noCache cache = tempDir{}

type tempDir struct{}

func (t tempDir) withLock(remote string, f func(dir string) error) error {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	return f(dir)
}

type cacheDir struct {
	dir string
}

func (c cacheDir) withLock(remote string, f func(dir string) error) error {
	h := sha256.New()
	io.WriteString(h, remote)
	hash := hex.EncodeToString(h.Sum(nil))
	dir := filepath.Join(c.dir, "src", hash)

	lockFile := dir + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockFile), 0755); err != nil {
		return err
	}

	closer, err := lock.Lock(lockFile)
	if err != nil {
		return fmt.Errorf("could not create lock file for remote %s, is another process downloading that package? (%v)", remote, err)
	}
	defer closer.Close()

	return f(dir)
}
