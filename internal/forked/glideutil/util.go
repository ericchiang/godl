package glideutil

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/Masterminds/vcs"
)

func init() {
	// Precompile the regular expressions used to check VCS locations.
	for _, v := range vcsList {
		v.regex = regexp.MustCompile(v.pattern)
	}
}

func toSlash(v string) string {
	return strings.Replace(v, "\\", "/", -1)
}

// GetRootFromPackage retrives the top level package from a name.
//
// From a package name find the root repo. For example,
// the package github.com/Masterminds/cookoo/io has a root repo
// at github.com/Masterminds/cookoo
func GetRootFromPackage(pkg string) (string, error) {
	pkg = toSlash(pkg)
	for _, v := range vcsList {
		m := v.regex.FindStringSubmatch(pkg)
		if m == nil {
			continue
		}

		if m[1] != "" {
			return m[1], nil
		}
	}

	// There are cases where a package uses the special go get magic for
	// redirects. If we've not discovered the location already try that.
	return getRootFromGoGet(pkg)
}

// Pages like https://golang.org/x/net provide an html document with
// meta tags containing a location to work with. The go tool uses
// a meta tag with the name go-import which is what we use here.
// godoc.org also has one call go-source that we do not need to use.
// The value of go-import is in the form "prefix vcs repo". The prefix
// should match the vcsURL and the repo is a location that can be
// checked out. Note, to get the html document you you need to add
// ?go-get=1 to the url.
func getRootFromGoGet(pkg string) (string, error) {

	p, found := checkRemotePackageCache(pkg)
	if found {
		return p, nil
	}

	vcsURL := "https://" + pkg
	u, err := url.Parse(vcsURL)
	if err != nil {
		return "", err
	}
	if u.RawQuery == "" {
		u.RawQuery = "go-get=1"
	} else {
		u.RawQuery = u.RawQuery + "&go-get=1"
	}
	checkURL := u.String()
	resp, err := http.Get(checkURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("GET %s %v", u, err)
	}

	nu, err := parseImportFromBody(u, resp.Body)
	if err != nil {
		return "", err
	}
	if nu == "" {
		addToRemotePackageCache(pkg, pkg)
		return pkg, nil
	}

	addToRemotePackageCache(pkg, nu)
	return nu, nil
}

var (
	remotePackageMu    = new(sync.RWMutex)
	remotePackageCache = make(map[string]string)
)

func checkRemotePackageCache(pkg string) (string, bool) {
	remotePackageMu.RLock()
	defer remotePackageMu.RUnlock()
	for k, v := range remotePackageCache {
		if pkg == k || strings.HasPrefix(pkg, k+"/") {
			return v, true
		}
	}

	return pkg, false
}

func addToRemotePackageCache(pkg, v string) {
	remotePackageMu.Lock()
	defer remotePackageMu.Unlock()
	remotePackageCache[pkg] = v
}

func parseImportFromBody(ur *url.URL, r io.ReadCloser) (u string, err error) {
	d := xml.NewDecoder(r)
	d.CharsetReader = charsetReader
	d.Strict = false
	var t xml.Token
	for {
		t, err = d.Token()
		if err != nil {
			if err == io.EOF {
				// If we hit the end of the markup and don't have anything
				// we return an error.
				err = vcs.ErrCannotDetectVCS
			}
			return
		}
		if e, ok := t.(xml.StartElement); ok && strings.EqualFold(e.Name.Local, "body") {
			return
		}
		if e, ok := t.(xml.EndElement); ok && strings.EqualFold(e.Name.Local, "head") {
			return
		}
		e, ok := t.(xml.StartElement)
		if !ok || !strings.EqualFold(e.Name.Local, "meta") {
			continue
		}
		if attrValue(e.Attr, "name") != "go-import" {
			continue
		}
		if f := strings.Fields(attrValue(e.Attr, "content")); len(f) == 3 {

			// If the prefix supplied by the remote system isn't a prefix to the
			// url we're fetching return continue looking for more go-imports.
			// This will work for exact matches and prefixes. For example,
			// golang.org/x/net as a prefix will match for golang.org/x/net and
			// golang.org/x/net/context.
			vcsURL := ur.Host + ur.Path
			if !strings.HasPrefix(vcsURL, f[0]) {
				continue
			} else {
				u = f[0]
				return
			}

		}
	}
}

func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(charset) {
	case "ascii":
		return input, nil
	default:
		return nil, fmt.Errorf("can't decode XML document using charset %q", charset)
	}
}

func attrValue(attrs []xml.Attr, name string) string {
	for _, a := range attrs {
		if strings.EqualFold(a.Name.Local, name) {
			return a.Value
		}
	}
	return ""
}

type vcsInfo struct {
	host    string
	pattern string
	regex   *regexp.Regexp
}

var vcsList = []*vcsInfo{
	{
		host:    "github.com",
		pattern: `^(?P<rootpkg>github\.com/[A-Za-z0-9_.\-]+/[A-Za-z0-9_.\-]+)(/[A-Za-z0-9_.\-]+)*$`,
	},
	{
		host:    "bitbucket.org",
		pattern: `^(?P<rootpkg>bitbucket\.org/([A-Za-z0-9_.\-]+/[A-Za-z0-9_.\-]+))(/[A-Za-z0-9_.\-]+)*$`,
	},
	{
		host:    "launchpad.net",
		pattern: `^(?P<rootpkg>launchpad\.net/(([A-Za-z0-9_.\-]+)(/[A-Za-z0-9_.\-]+)?|~[A-Za-z0-9_.\-]+/(\+junk|[A-Za-z0-9_.\-]+)/[A-Za-z0-9_.\-]+))(/[A-Za-z0-9_.\-]+)*$`,
	},
	{
		host:    "git.launchpad.net",
		pattern: `^(?P<rootpkg>git\.launchpad\.net/(([A-Za-z0-9_.\-]+)|~[A-Za-z0-9_.\-]+/(\+git|[A-Za-z0-9_.\-]+)/[A-Za-z0-9_.\-]+))$`,
	},
	{
		host:    "hub.jazz.net",
		pattern: `^(?P<rootpkg>hub\.jazz\.net/git/[a-z0-9]+/[A-Za-z0-9_.\-]+)(/[A-Za-z0-9_.\-]+)*$`,
	},
	{
		host:    "go.googlesource.com",
		pattern: `^(?P<rootpkg>go\.googlesource\.com/[A-Za-z0-9_.\-]+/?)$`,
	},
	// TODO: Once Google Code becomes fully deprecated this can be removed.
	{
		host:    "code.google.com",
		pattern: `^(?P<rootpkg>code\.google\.com/[pr]/([a-z0-9\-]+)(\.([a-z0-9\-]+))?)(/[A-Za-z0-9_.\-]+)*$`,
	},
	// Alternative Google setup for SVN. This is the previous structure but it still works... until Google Code goes away.
	{
		pattern: `^(?P<rootpkg>[a-z0-9_\-.]+\.googlecode\.com/svn(/.*)?)$`,
	},
	// Alternative Google setup. This is the previous structure but it still works... until Google Code goes away.
	{
		pattern: `^(?P<rootpkg>[a-z0-9_\-.]+\.googlecode\.com/(git|hg))(/.*)?$`,
	},
	// If none of the previous detect the type they will fall to this looking for the type in a generic sense
	// by the extension to the path.
	{
		pattern: `^(?P<rootpkg>(?P<repo>([a-z0-9.\-]+\.)+[a-z0-9.\-]+(:[0-9]+)?/[A-Za-z0-9_.\-/]*?)\.(bzr|git|hg|svn))(/[A-Za-z0-9_.\-]+)*$`,
	},
}
