package main

import (
	"fmt"
	"os"
	"testing"
	"time"
)

type fakeFile struct {
	name string
	mode os.FileMode
}

func (f *fakeFile) Name() string       { return f.name }
func (f *fakeFile) Size() int64        { return 0 }
func (f *fakeFile) Mode() os.FileMode  { return f.mode }
func (f *fakeFile) ModTime() time.Time { return time.Now() }
func (f *fakeFile) IsDir() bool        { return f.mode.IsDir() }
func (f *fakeFile) Sys() interface{}   { return nil }

func (f *fakeFile) String() string {
	return fmt.Sprintf("%s %s", f.name, f.mode)
}

func TestIgnore(t *testing.T) {
	tests := []struct {
		name string
		mode os.FileMode
		want bool
	}{
		{
			name: "hello.go",
		},
		{
			// Ignore test files.
			name: "hello_test.go",
			want: true,
		},
		{
			name: "dir",
			mode: os.ModeDir,
		},
		{
			// Don't want to ignore c files.
			name: "foo.c",
		},
		{
			// Don't want to ignore assembly files.
			name: "foo.s",
		},
		{
			name: "README.md",
			want: true,
		},
		{
			name: "symlink.go",
			mode: os.ModeSymlink,
			want: true,
		},
		{
			name: "LICENSE",
		},
	}
	for _, test := range tests {
		info := &fakeFile{test.name, test.mode}
		got := ignore(info)
		if test.want != got {
			t.Errorf("ignore(%s), want=%t, got=%t", info, test.want, got)
		}
	}
}
