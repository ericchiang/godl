# godl - Download projects to you vendor directory

godl is a Go vendoring tool that optimizes for making small changes to a project's dependencies (e.g. updating a single dependency).

Unlike other tools, godl doesn't do analysis of dependencies, inspect source files, or interact with your GOPATH. Packages are downloaded one at a time so you can modify a single dependency without performing expensive operations like re-downloading all packages or performing static analysis on a large repo.

```terminal
godl init
godl get golang.org/x/net feeb485667d1fdabe727840fe00adc22431bc86e
godl get gopkg.in/square/go-jose.v2 v2.1.0
godl get github.com/spf13/cobra # Default's to latest
```

## FAQ

Q: Since godl won't do it for me, how do I list all my projects dependencies?

A: Use go list.

```terminal
go list -f '{{.Deps}}' | tr "[" " " | tr "]" " " | xargs go list -f '{{if not .Standard}}{{.ImportPath}}{{end}}'
```

Q: How do I download private repos?

A: Use the `--remote` flag to manually specify the remote repo.

```terminal
godl get gopkg.in/square/go-jose.v2 v2.1.0 --remote git@github.com:square/go-jose.git
```

Subsequent calls to `godl get` that omit the `--remote` flag will default to the previous value.
