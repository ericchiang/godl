package download

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
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
		{name: "hello.go"},
		{name: "hello_test.go", want: true},
		{name: "dir", mode: os.ModeDir},
		{name: "foo.c"},
		{name: "foo.s"},
		{name: "README.md", want: true},
		{name: "symlink.go", mode: os.ModeSymlink, want: true},
		{name: "LICENSE"},
	}
	for _, test := range tests {
		info := &fakeFile{test.name, test.mode}
		got := ignore(info)
		if test.want != got {
			t.Errorf("ignore(%s), want=%t, got=%t", info, test.want, got)
		}
	}
}

func TestCopyFile(t *testing.T) {
	p := "foo/bar.go"
	test := copyTest{
		files: []fakefile{
			{"foo/bar.go", ""},
			{"foo/foo.go", ""},
		},
		want: []string{"foo/bar.go"},
		copyFiles: func(t *testing.T, dest, src string) {
			destFile := filepath.Join(dest, p)
			srcFile := filepath.Join(src, p)

			info, err := os.Stat(srcFile)
			if err != nil {
				t.Fatal(err)
			}
			if err := copyFile(destFile, srcFile, info); err != nil {
				t.Fatal(err)
			}
		},
	}
	test.run(t)
}

type copyTest struct {
	files []fakefile
	want  []string

	copyFiles func(t *testing.T, dest, src string)
}

func (c copyTest) run(t *testing.T) {
	src, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(src)

	dest, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dest)

	for _, file := range c.files {
		if err := file.write(src); err != nil {
			t.Fatal(err)
		}
	}

	c.copyFiles(t, dest, src)

	got, err := listFilepaths(dest)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	sort.Strings(c.want)

	if !reflect.DeepEqual(got, c.want) {
		t.Errorf("expected files %q got %q", c.want, got)
	}
}

type fakefile struct {
	path     string
	contents string
}

func (f fakefile) write(dir string) error {
	p := filepath.Join(dir, f.path)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}

	return ioutil.WriteFile(p, []byte(f.contents), 0644)
}

func listFilepaths(dir string) ([]string, error) {
	var files []string
	f := func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	}

	return files, filepath.Walk(dir, f)
}
