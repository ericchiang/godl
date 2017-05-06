# godl - Download projects to you vendor directory

godl is an inconvenient tool for managing your vendor directory. You probably shouldn't use it.

Unlike other tools, godl doesn't do analysis of dependencies, inspect source files, or interact with your GOPATH. It is simply takes a list of expected package versions and downloads them to your vendor directory. Packages are downloaded one at a time so you can modify a single dependency without performing potentially expensive operations.

```
godl init
godl get golang.org/x/net feeb485667d1fdabe727840fe00adc22431bc86e
godl get github.com/spf13/pflag 80fe0fb4eba54167e2ccae1c6c950e72abf61b73
godl get github.com/spf13/cobra db6b9a8b3f3f400c8ecb4a4d7d02245b8facad66
godl get gopkg.in/square/go-jose.v2 v2.1.0
```

godl can also verify that it was the tool used to create the contents of the vendor directory. This allows project owners to ensure contributors took the correct steps when modifying the vendor directory.

```
godl verify
```

## Project status

Extremely experimental.
